# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import numpy as np
import pandas as pd
from optflow.workflow.runner.custom_solver_runner import CustomSolverRunner
from sqlflow_submitter.optimize.optimize import generate_model_with_data_frame


class BaseOptFlowRunner(CustomSolverRunner):
    def init_parameters(self):
        raise NotImplementedError()

    def _create_model(self, data_frame):
        model = generate_model_with_data_frame(
            data_frame=data_frame,
            variables=self.variables,
            variable_type=self.variable_type,
            result_value_name=self.result_value_name,
            objective=self.objective,
            direction=self.direction,
            constraints=self.constraints)
        return model

    def _is_integer_type(self):
        return self.variable_type.endswith('Integers')

    def _get_variable_columns(self, data_frame):
        result_columns = []
        lower_variables = [v.lower() for v in self.variables]
        for c in data_frame.columns:
            if c.lower() in lower_variables:
                result_columns.append(c)

        if len(self.variables) == 1 and self.result_value_name.lower(
        ) == self.variables[0].lower():
            result_column = self.result_value_name + "_value"
        else:
            result_column = self.result_value_name
        return result_columns, result_column

    def solver_run(self):
        self.init_parameters()

        models = []
        columns = None
        result_column = None
        output = None
        dtype = np.int64 if self._is_integer_type() else np.float64

        if isinstance(self.input_dfs.df1, pd.DataFrame):
            data_frames = [self.input_dfs.df1]
        else:
            data_frames = self.input_dfs.df1

        for batch_index, df in enumerate(data_frames):
            print("Input data is \n", df)

            # step 1: build model
            model = self._create_model(df)
            models.append(model)

            # step 2: solve model
            model = self.solve_model(model)
            model.display()

            if batch_index == 0:
                columns, result_column = self._get_variable_columns(df)
                output = pd.DataFrame(
                    columns=columns +
                    ["batch_index", "worker_index", result_column])

            var_num = len(df)
            result_data_dict = {}
            for c in columns:
                result_data_dict[c] = df[c]

            result_data_dict["batch_index"] = np.full(shape=[var_num],
                                                      fill_value=batch_index,
                                                      dtype=dtype)
            result_data_dict["worker_index"] = np.full(
                shape=[var_num],
                fill_value=self.context.worker_index,
                dtype=dtype)
            result_data_dict[result_column] = np.array(
                [model.x[i]() for i in model.x], dtype=dtype)

            result_data = pd.DataFrame(data=result_data_dict)
            output = output.append(result_data, ignore_index=True)

        print('Output data is \n', output)
        output_dfs = {'df1': output}
        self.dump_outputs(output_dfs)
