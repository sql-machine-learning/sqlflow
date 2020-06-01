#!/bin/bash

# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

. /find_fastest_resources.sh

echo "Find fastest apt-get mirror ..."
$(find_fastest_apt_source > /etc/apt/sources.list)

apt-get -qq update

echo "Setup pip mirro ..."
mkdir -p ~/.pip
$(find_fastest_pip_mirror > ~/.pip/pip.conf)

