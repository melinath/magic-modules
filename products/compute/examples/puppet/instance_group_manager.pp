<%# The license inside this block applies to this file
# Copyright 2017 Google Inc.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
-%>
<% unless name == "README.md" -%>
<%= compile 'templates/license.erb' -%>

<%= lines(autogen_notice :puppet) -%>

<%= compile 'templates/puppet/examples~credential.pp.erb' -%>

gcompute_zone { 'us-west1-a':
  project    => $project, # e.g. 'my-test-project'
  credential => 'mycred',
}

gcompute_machine_type { 'n1-standard-1':
  project    => $project, # e.g. 'my-test-project'
  zone       => 'us-west1-a',
  credential => 'mycred',
}

gcompute_network { <%= example_resource_name('mynetwork-test') -%>:
  ensure     => present,
  project    => $project, # e.g. 'my-test-project'
  credential => 'mycred',
}

gcompute_instance_template { <%= example_resource_name('instance-template') -%>:
  ensure     => present,
  properties => {
    machine_type       => 'n1-standard-1',
    disks              => [
      {
        # Tip: Auto delete will prevent disks from being left behind on
        # deletion.
        auto_delete       => true,
        boot              => true,
        initialize_params => {
          source_image =>
            gcompute_image_family('ubuntu-1604-lts', 'ubuntu-os-cloud'),
        }
      }
    ],
    network_interfaces => [
      {
        access_configs => [
          {
          name => 'External NAT',
          type => 'ONE_TO_ONE_NAT',
          },
        ],
        network        => <%= example_resource_name('mynetwork-test') %>,
      }
    ]
  },
  project    => $project, # e.g. 'my-test-project'
  credential => 'mycred',
}

<% end # name == README.md -%>
gcompute_instance_group_manager { <%= example_resource_name('test1') -%>:
  ensure             => present,
  base_instance_name => <%= example_resource_name('test1-child') -%>,
  instance_template  => <%= example_resource_name('instance-template') -%>,
  target_size        => 3,
  zone               => 'us-west1-a',
  project            => $project, # e.g. 'my-test-project'
  credential         => 'mycred',
}
