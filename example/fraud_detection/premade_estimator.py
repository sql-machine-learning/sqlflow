#  Copyright 2016 The TensorFlow Authors. All Rights Reserved.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
"""An Example of a DNNClassifier for the Iris dataset."""
from __future__ import absolute_import
from __future__ import division
from __future__ import print_function

import argparse
import tensorflow as tf
# import tensorflow.contrib.eager as tfe
# tf.enable_eager_execution()

import creditcard_data


parser = argparse.ArgumentParser()
parser.add_argument('--batch_size', default=100, type=int, help='batch size')
parser.add_argument('--train_steps', default=1000, type=int,
                    help='number of training steps')

def main(argv):
    args = parser.parse_args(argv[1:])

    # Fetch the data
    (train_x, train_y), (test_x, test_y) = creditcard_data.load_data()

    # Feature columns describe how to use the input.
    my_feature_columns = []
    for key in train_x.keys():
        print("Adding numeric_columns: {}".format(key))
        my_feature_columns.append(tf.feature_column.numeric_column(key=key))

    # Build 2 hidden layer DNN with 100, 100 units respectively.
    classifier = tf.estimator.DNNClassifier(
        feature_columns=my_feature_columns,
        # Two hidden layers of 10 nodes each.
        hidden_units=[100, 100],
        # The model must choose between 3 classes.
        n_classes=2)

    # Train the Model.
    classifier.train(
        input_fn=lambda:creditcard_data.train_input_fn(train_x, train_y,
                                                 args.batch_size),
        steps=args.train_steps)

    # Evaluate the model.
    eval_result = classifier.evaluate(
        input_fn=lambda:creditcard_data.eval_input_fn(test_x, test_y,
                                                args.batch_size))

    print('\nTest set accuracy: {accuracy:0.5f}\n'.format(**eval_result))

    # # Generate predictions from the model
    expected = [0, 0, 0]
    predict_x = {
        'Time': [80.0, 80.0, 81.0],
        'V1': [-0.655264276212716, -3.00773905109524, 1.11104797028704],
        'V2': [0.40989854283685295, 0.929514494266081, 0.215351844382971],
        'V3': [1.28915643808596, -0.42372085549803296, 0.374072563622289],
        'V4': [-0.32504325923176003, -1.3929516201707801, 1.1500006534851999],
        'V5': [0.545669467098008, 0.0822467824081475, -0.329767568535767],
        'V6': [-0.349810664410475, -0.515781701182148, -0.898687821706977],
        'V7': [0.648240481592264, 0.104713792402636, 0.30990664398233],
        'V8': [0.0360628816033704, 0.8253105658956079, -0.25527729116611103],
        'V9': [0.0787014163023305, 0.38545311035542207, -0.110655138734984],
        'V10': [-0.829167267726513, 0.0640002245721828, -0.0575788321387636],
        'V11': [-1.78165873189534, -1.29310671179606, -0.21796149670612197],
        'V12': [-0.412806782520344, 0.0621999429338998, 0.600591209357443],
        'V13': [-0.6262202895658521, 0.27110701307160395, 0.686379564975428],
        'V14': [-0.113361054850318, 0.328058800506629, 0.262161286299107],
        'V15': [-0.714754632820629, 0.7240976288015201, 0.9226769790285508],
        'V16': [0.120701513866132, 0.6383894438325529, 0.135235271944387],
        'V17': [-0.47428167431379603, -0.43008221783924894, -0.490174913186185],
        'V18': [-0.240594669556031, -0.7235902240912719, -0.392752898979319],
        'V19': [0.0408234763497192, -0.562830692998269, -0.315464926788021],
        'V20': [-0.0347759899640066, -0.15171422957684697, 0.031671787802255],
        'V21': [-0.155727058998072, -0.40924608012503005, -0.132555303999709],
        'V22': [-0.47739031832143397, -0.588831627173022, -0.493866878067896],
        'V23': [-0.126524992708454, 0.0680450889350803, -0.0107821837368997],
        'V24': [-0.443627776856693, -0.890869739213917, 0.380388564169084],
        'V25': [-0.012118142098906, -0.047641092795032, 0.45621418060635105],
        'V26': [0.143172915176852, 0.717616984694838, -0.572346756048474],
        'V27': [0.049782585303356, -0.0540936594394297, 0.0109814630026346],
        'V28': [0.118280286710879, -0.42425971753603997, 0.039295013148069],
        'Amount': [32.51, 39.95, 68.74]
    }

    predictions = classifier.predict(
        input_fn=lambda:creditcard_data.eval_input_fn(features=predict_x,
                                                labels=None,
                                                batch_size=args.batch_size))

    template = ('\nPrediction is "{}" ({:.1f}%), expected "{}"')
    for pred_dict, expec in zip(predictions, expected):
        class_id = pred_dict['class_ids'][0]
        probability = pred_dict['probabilities'][class_id]
        print(template.format(["Not Fraud", "Fraud"][class_id],
                              100 * probability, ["Not Fraud", "Fraud"][expec]))


if __name__ == '__main__':
    tf.logging.set_verbosity(tf.logging.INFO)
    tf.app.run(main)
