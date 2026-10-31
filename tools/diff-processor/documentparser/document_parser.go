package documentparser

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	// A field bullet at the start of a line, optionally indented: * `xxx` or - `xxx`
	fieldNameRegex = regexp.MustCompile("(?m)^([ \\t]*)[*-][ \\t]+`([a-z0-9_./]+)`")
	// More field names after the first in a bullet: * `xxx` or `yyy`
	moreNamesRegex = regexp.MustCompile("^(?:[ \\t]*(?:,|/|or|and|,[ \\t]*or|,[ \\t]*and)[ \\t]*`[a-z0-9_.]+`)+")
	// A field name in a bullet.
	blockNameRegex      = regexp.MustCompile("`([a-z0-9_.]+)(?:\\[\\])?`")
	nestedObjectRegex   = regexp.MustCompile(`<a\s+name="([a-z0-9_]+)">`) // <a name="xxx">
	nestedHashTagRegex  = regexp.MustCompile(`\(#(nested_[a-z0-9_]+)\)`)  // #(nested_xxx)
	horizontalLineRegex = regexp.MustCompile("- - -|-{3,}")               // - - - or ---
	// A level 2 heading.
	sectionHeadingRegex = regexp.MustCompile(`(?m)^##[ \t]+(.*)$`)
	// The level 2 headings of the sections that document fields.
	fieldSectionRegex = regexp.MustCompile(`(?i)^(?:arguments?|attributes?|ephemeral attributes?) reference`)
	// An index in a field path written in the a.0.b format.
	indexRegex = regexp.MustCompile(`\.[0-9]+(\.|$)`)
)

const maxDepth = 20

// DocumentParser parse *.html.markdown resource doc files.
type DocumentParser struct {
	root        *node
	nestedBlock map[string]string
}

type node struct {
	name     string
	children []*node
}

func NewParser() *DocumentParser {
	return &DocumentParser{
		nestedBlock: make(map[string]string),
	}
}

func (d *DocumentParser) FlattenFields() []string {
	var paths []string
	traverse(
		&paths,
		"",
		d.root,
	)
	// A field path such as `a.0.b` documents its parent fields.
	for _, path := range paths {
		for i := range path {
			if path[i] == '.' {
				paths = append(paths, path[:i])
			}
		}
	}
	sort.Strings(paths)
	return slices.Compact(paths)
}

func traverse(paths *[]string, path string, n *node) {
	if n == nil {
		return
	}
	var curPath string
	if path != "" {
		curPath = path + "." + n.name
	} else {
		curPath = n.name
	}
	if curPath != "" {
		*paths = append(*paths, curPath)
	}
	for _, c := range n.children {
		traverse(paths, curPath, c)
	}
}

// Parse parse a resource document markdown's arguments and attributes section.
// The parsed file format is defined in mmv1/templates/terraform/resource.html.markdown.tmpl.
func (d *DocumentParser) Parse(src []byte) error {
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	headings := sectionHeadingRegex.FindAllStringSubmatchIndex(text, -1)
	var topLevel []string
	for i, h := range headings {
		if !fieldSectionRegex.MatchString(text[h[2]:h[3]]) {
			continue
		}
		end := len(text)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		section := text[h[1]:end]
		for _, part := range horizontalLineRegex.Split(section, -1) {
			topLevelPropertySection, err := d.extractNestedObject(part)
			if err != nil {
				return err
			}
			topLevel = append(topLevel, topLevelPropertySection)
		}
	}
	d.root = &node{}
	for _, t := range topLevel {
		d.root.children = append(d.root.children, d.buildNodes(t, 0, map[string]bool{})...)
	}
	return nil
}

func (d *DocumentParser) extractNestedObject(input string) (string, error) {
	parts := splitWithRegexp(input, nestedObjectRegex)
	for _, p := range parts[1:] {
		nestedName := findPattern(p, nestedObjectRegex)
		if nestedName == "" {
			return "", fmt.Errorf("could not find nested object name in %s", p)
		}
		d.nestedBlock[nestedName] = p
	}
	return parts[0], nil
}

type bullet struct {
	names    []string
	indent   int
	ownText  string
	fullText string
	children []*bullet
}

func indentation(s string) int {
	return len(strings.ReplaceAll(s, "\t", "    "))
}

// Parse the field bullets in the text into a tree, using indentation for
// nesting. Each line of text belongs to the innermost open bullet. An indented
// bullet ends at a line that isn't indented more than it, and any bullet ends at
// a new paragraph that isn't indented more than it.
func parseBullets(text string) []*bullet {
	matches := fieldNameRegex.FindAllStringSubmatchIndex(text, -1)
	var roots []*bullet
	var stack []*bullet
	for i, m := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		lines := strings.SplitAfter(text[m[0]:end], "\n")
		indent := indentation(text[m[2]:m[3]])
		if indent > 0 && len(lines) > 1 && strings.HasSuffix(strings.TrimSpace(lines[0]), "-") {
			if trimmed := strings.TrimLeft(lines[1], " \t"); strings.TrimSpace(trimmed) != "" {
				if nextIndent := indentation(lines[1][:len(lines[1])-len(trimmed)]); nextIndent <= indent {
					indent = 0
				}
			}
		}
		b := &bullet{
			names:   []string{text[m[4]:m[5]]},
			indent:  indent,
			ownText: lines[0],
		}
		if more := moreNamesRegex.FindString(text[m[1]:end]); more != "" {
			for _, n := range blockNameRegex.FindAllStringSubmatch(more, -1) {
				b.names = append(b.names, n[1])
			}
		}
		for len(stack) > 0 && stack[len(stack)-1].indent >= b.indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, b)
		} else {
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, b)
		}
		stack = append(stack, b)
		prevBlank := false
		for _, line := range lines[1:] {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.TrimSpace(trimmed) == "" {
				if len(stack) > 0 {
					stack[len(stack)-1].ownText += line
				}
				prevBlank = true
				continue
			}
			ind := indentation(line[:len(line)-len(trimmed)])
			for len(stack) > 0 {
				top := stack[len(stack)-1]
				if ind <= top.indent && (top.indent > 0 || prevBlank) {
					stack = stack[:len(stack)-1]
				} else {
					break
				}
			}
			if len(stack) > 0 {
				stack[len(stack)-1].ownText += line
			}
			prevBlank = false
		}
	}
	var fill func(b *bullet) string
	fill = func(b *bullet) string {
		b.fullText = b.ownText
		for _, c := range b.children {
			b.fullText += fill(c)
		}
		return b.fullText
	}
	for _, r := range roots {
		fill(r)
	}
	return roots
}

func normalize(name string) string {
	for indexRegex.MatchString(name) {
		name = indexRegex.ReplaceAllString(name, "$1")
	}
	return strings.TrimSuffix(name, ".")
}

func (d *DocumentParser) buildNodes(text string, depth int, visited map[string]bool) []*node {
	if depth > maxDepth {
		return nil
	}
	var build func(bs []*bullet) []*node
	build = func(bs []*bullet) []*node {
		var nodes []*node
		for _, b := range bs {
			for _, name := range b.names {
				name = normalize(name)
				n := &node{name: name}
				n.children = build(b.children)
				nestedHashTag := findPattern(b.ownText, nestedHashTagRegex)
				if nestedHashTag == "" {
					nestedHashTag = findPattern(b.fullText, nestedHashTagRegex)
				}
				if block, ok := d.nestedBlock[nestedHashTag]; ok && !visited[nestedHashTag] {
					v := map[string]bool{nestedHashTag: true}
					for k := range visited {
						v[k] = true
					}
					n.children = append(n.children, d.buildNodes(block, depth+1, v)...)
				}
				nodes = append(nodes, n)
			}
		}
		return nodes
	}
	return build(parseBullets(text))
}

func findPattern(text string, re *regexp.Regexp) string {
	match := re.FindStringSubmatch(text)
	if match != nil {
		return match[1]
	}
	return ""
}

func splitWithRegexp(text string, re *regexp.Regexp) []string {
	matches := re.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return []string{text}
	}
	var parts []string
	start := 0
	for _, match := range matches {
		end := match[0]

		parts = append(parts, text[start:end])
		start = end
	}
	parts = append(parts, text[start:])
	return parts
}
