// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

require(['notebook/js/codecell', "codemirror/lib/codemirror"], function(codecell, CodeMirror) {
    CodeMirror.modeInfo.push({name: "MySQLFlow", mime: "text/x-mysqlflow", mode: "sqlflow"})
    codecell.CodeCell.options_default.highlight_modes['magic_text/x-mysqlflow'] = {'reg':[/^%%sqlflow/]} ;
    Jupyter.notebook.events.one('kernel_ready.Kernel', function(){
        Jupyter.notebook.get_cells().map(function(cell){
            if (cell.cell_type == 'code'){
                cell.auto_highlight();
            }
        });
    });
});
