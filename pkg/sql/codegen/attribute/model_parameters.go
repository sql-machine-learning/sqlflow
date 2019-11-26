// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

// Code generated by python extract_docstring.py > model_parameters.go. DO NOT EDIT.

package attribute

const ModelParameterJSON = `
{
    "DNNClassifier": {
        "hidden_units": "Iterable of number hidden units per layer. All layers are fully connected. Ex. '[64, 32]' means first layer has 64 nodes and second one has 32.",
        "feature_columns": "An iterable containing all the feature columns used by the model. All items in the set should be instances of classes derived from '_FeatureColumn'.",
        "model_dir": "Directory to save model parameters, graph and etc. This can also be used to load checkpoints from the directory into a estimator to continue training a previously saved model.",
        "n_classes": "Number of label classes. Defaults to 2, namely binary classification. Must be > 1.",
        "weight_column": "A string or a '_NumericColumn' created by 'tf.feature_column.numeric_column' defining feature column representing weights. It is used to down weight or boost examples during training. It will be multiplied by the loss of the example. If it is a string, it is used as a key to fetch weight tensor from the 'features'. If it is a '_NumericColumn', raw tensor is fetched by key 'weight_column.key', then weight_column.normalizer_fn is applied on it to get weight tensor.",
        "label_vocabulary": "A list of strings represents possible label values. If given, labels must be string type and have any value in 'label_vocabulary'. If it is not given, that means labels are already encoded as integer or float within [0, 1] for 'n_classes=2' and encoded as integer values in {0, 1,..., n_classes-1} for 'n_classes'>2 . Also there will be errors if vocabulary is not provided and labels are string.",
        "optimizer": "An instance of 'tf.Optimizer' used to train the model. Can also be a string (one of 'Adagrad', 'Adam', 'Ftrl', 'RMSProp', 'SGD'), or callable. Defaults to Adagrad optimizer.",
        "activation_fn": "Activation function applied to each layer. If 'None', will use 'tf.nn.relu'.",
        "dropout": "When not 'None', the probability we will drop out a given coordinate.",
        "config": "'RunConfig' object to configure the runtime settings.",
        "warm_start_from": "A string filepath to a checkpoint to warm-start from, or a 'WarmStartSettings' object to fully configure warm-starting. If the string filepath is provided instead of a 'WarmStartSettings', then all weights are warm-started, and it is assumed that vocabularies and Tensor names are unchanged.",
        "loss_reduction": "One of 'tf.losses.Reduction' except 'NONE'. Describes how to reduce training loss over batch. Defaults to 'SUM_OVER_BATCH_SIZE'.",
        "batch_norm": "Whether to use batch normalization after each hidden layer."
    },
    "DNNRegressor": {
        "hidden_units": "Iterable of number hidden units per layer. All layers are fully connected. Ex. '[64, 32]' means first layer has 64 nodes and second one has 32.",
        "feature_columns": "An iterable containing all the feature columns used by the model. All items in the set should be instances of classes derived from '_FeatureColumn'.",
        "model_dir": "Directory to save model parameters, graph and etc. This can also be used to load checkpoints from the directory into a estimator to continue training a previously saved model.",
        "label_dimension": "Number of regression targets per example. This is the size of the last dimension of the labels and logits 'Tensor' objects (typically, these have shape '[batch_size, label_dimension]').",
        "weight_column": "A string or a '_NumericColumn' created by 'tf.feature_column.numeric_column' defining feature column representing weights. It is used to down weight or boost examples during training. It will be multiplied by the loss of the example. If it is a string, it is used as a key to fetch weight tensor from the 'features'. If it is a '_NumericColumn', raw tensor is fetched by key 'weight_column.key', then weight_column.normalizer_fn is applied on it to get weight tensor.",
        "optimizer": "An instance of 'tf.keras.optimizers.Optimizer' used to train the model. Can also be a string (one of 'Adagrad', 'Adam', 'Ftrl', 'RMSProp', 'SGD'), or callable. Defaults to Adagrad optimizer.",
        "activation_fn": "Activation function applied to each layer. If 'None', will use 'tf.nn.relu'.",
        "dropout": "When not 'None', the probability we will drop out a given coordinate.",
        "config": "'RunConfig' object to configure the runtime settings.",
        "warm_start_from": "A string filepath to a checkpoint to warm-start from, or a 'WarmStartSettings' object to fully configure warm-starting. If the string filepath is provided instead of a 'WarmStartSettings', then all weights are warm-started, and it is assumed that vocabularies and Tensor names are unchanged.",
        "loss_reduction": "One of 'tf.losses.Reduction' except 'NONE'. Describes how to reduce training loss over batch. Defaults to 'SUM_OVER_BATCH_SIZE'.",
        "batch_norm": "Whether to use batch normalization after each hidden layer."
    },
    "LinearClassifier": {
        "feature_columns": "An iterable containing all the feature columns used by the model. All items in the set should be instances of classes derived from 'FeatureColumn'.",
        "model_dir": "Directory to save model parameters, graph and etc. This can also be used to load checkpoints from the directory into a estimator to continue training a previously saved model.",
        "n_classes": "number of label classes. Default is binary classification. Note that class labels are integers representing the class index (i.e. values from 0 to n_classes-1). For arbitrary label values (e.g. string labels), convert to class indices first.",
        "weight_column": "A string or a '_NumericColumn' created by 'tf.feature_column.numeric_column' defining feature column representing weights. It is used to down weight or boost examples during training. It will be multiplied by the loss of the example. If it is a string, it is used as a key to fetch weight tensor from the 'features'. If it is a '_NumericColumn', raw tensor is fetched by key 'weight_column.key', then weight_column.normalizer_fn is applied on it to get weight tensor.",
        "label_vocabulary": "A list of strings represents possible label values. If given, labels must be string type and have any value in 'label_vocabulary'. If it is not given, that means labels are already encoded as integer or float within [0, 1] for 'n_classes=2' and encoded as integer values in {0, 1,..., n_classes-1} for 'n_classes'>2 . Also there will be errors if vocabulary is not provided and labels are string.",
        "optimizer": "An instance of 'tf.Optimizer' or 'tf.estimator.experimental.LinearSDCA' used to train the model. Can also be a string (one of 'Adagrad', 'Adam', 'Ftrl', 'RMSProp', 'SGD'), or callable. Defaults to FTRL optimizer.",
        "config": "'RunConfig' object to configure the runtime settings.",
        "warm_start_from": "A string filepath to a checkpoint to warm-start from, or a 'WarmStartSettings' object to fully configure warm-starting. If the string filepath is provided instead of a 'WarmStartSettings', then all weights and biases are warm-started, and it is assumed that vocabularies and Tensor names are unchanged.",
        "loss_reduction": "One of 'tf.losses.Reduction' except 'NONE'. Describes how to reduce training loss over batch. Defaults to 'SUM_OVER_BATCH_SIZE'.",
        "sparse_combiner": "A string specifying how to reduce if a categorical column is multivalent. One of \"mean\", \"sqrtn\", and \"sum\" -- these are effectively different ways to do example-level normalization, which can be useful for bag-of-words features. for more details, see 'tf.feature_column.linear_model'. Returns: A 'LinearClassifier' estimator. Raises: ValueError: if n_classes < 2."
    },
    "LinearRegressor": {
        "feature_columns": "An iterable containing all the feature columns used by the model. All items in the set should be instances of classes derived from 'FeatureColumn'.",
        "model_dir": "Directory to save model parameters, graph and etc. This can also be used to load checkpoints from the directory into a estimator to continue training a previously saved model.",
        "label_dimension": "Number of regression targets per example. This is the size of the last dimension of the labels and logits 'Tensor' objects (typically, these have shape '[batch_size, label_dimension]').",
        "weight_column": "A string or a '_NumericColumn' created by 'tf.feature_column.numeric_column' defining feature column representing weights. It is used to down weight or boost examples during training. It will be multiplied by the loss of the example. If it is a string, it is used as a key to fetch weight tensor from the 'features'. If it is a '_NumericColumn', raw tensor is fetched by key 'weight_column.key', then weight_column.normalizer_fn is applied on it to get weight tensor.",
        "optimizer": "An instance of 'tf.Optimizer' or 'tf.estimator.experimental.LinearSDCA' used to train the model. Can also be a string (one of 'Adagrad', 'Adam', 'Ftrl', 'RMSProp', 'SGD'), or callable. Defaults to FTRL optimizer.",
        "config": "'RunConfig' object to configure the runtime settings.",
        "warm_start_from": "A string filepath to a checkpoint to warm-start from, or a 'WarmStartSettings' object to fully configure warm-starting. If the string filepath is provided instead of a 'WarmStartSettings', then all weights and biases are warm-started, and it is assumed that vocabularies and Tensor names are unchanged.",
        "loss_reduction": "One of 'tf.losses.Reduction' except 'NONE'. Describes how to reduce training loss over batch. Defaults to 'SUM'.",
        "sparse_combiner": "A string specifying how to reduce if a categorical column is multivalent. One of \"mean\", \"sqrtn\", and \"sum\" -- these are effectively different ways to do example-level normalization, which can be useful for bag-of-words features. for more details, see 'tf.feature_column.linear_model'."
    },
    "BoostedTreesClassifier": {
        "feature_columns": "An iterable containing all the feature columns used by the model. All items in the set should be instances of classes derived from 'FeatureColumn'.",
        "n_batches_per_layer": "the number of batches to collect statistics per layer. The total number of batches is total number of data divided by batch size.",
        "model_dir": "Directory to save model parameters, graph and etc. This can also be used to load checkpoints from the directory into a estimator to continue training a previously saved model.",
        "n_classes": "number of label classes. Default is binary classification. Multiclass support is not yet implemented.",
        "weight_column": "A string or a 'NumericColumn' created by 'tf.fc_old.numeric_column' defining feature column representing weights. It is used to downweight or boost examples during training. It will be multiplied by the loss of the example. If it is a string, it is used as a key to fetch weight tensor from the 'features'. If it is a 'NumericColumn', raw tensor is fetched by key 'weight_column.key', then weight_column.normalizer_fn is applied on it to get weight tensor.",
        "label_vocabulary": "A list of strings represents possible label values. If given, labels must be string type and have any value in 'label_vocabulary'. If it is not given, that means labels are already encoded as integer or float within [0, 1] for 'n_classes=2' and encoded as integer values in {0, 1,..., n_classes-1} for 'n_classes'>2 . Also there will be errors if vocabulary is not provided and labels are string.",
        "n_trees": "number trees to be created.",
        "max_depth": "maximum depth of the tree to grow.",
        "learning_rate": "shrinkage parameter to be used when a tree added to the model.",
        "l1_regularization": "regularization multiplier applied to the absolute weights of the tree leafs.",
        "l2_regularization": "regularization multiplier applied to the square weights of the tree leafs.",
        "tree_complexity": "regularization factor to penalize trees with more leaves.",
        "min_node_weight": "min_node_weight: minimum hessian a node must have for a split to be considered. The value will be compared with sum(leaf_hessian)/(batch_size * n_batches_per_layer).",
        "config": "'RunConfig' object to configure the runtime settings.",
        "center_bias": "Whether bias centering needs to occur. Bias centering refers to the first node in the very first tree returning the prediction that is aligned with the original labels distribution. For example, for regression problems, the first node will return the mean of the labels. For binary classification problems, it will return a logit for a prior probability of label 1.",
        "pruning_mode": "one of 'none', 'pre', 'post' to indicate no pruning, pre- pruning (do not split a node if not enough gain is observed) and post pruning (build the tree up to a max depth and then prune branches with negative gain). For pre and post pruning, you MUST provide tree_complexity >0.",
        "quantile_sketch_epsilon": "float between 0 and 1. Error bound for quantile computation. This is only used for float feature columns, and the number of buckets generated per float feature is 1/quantile_sketch_epsilon.",
        "train_in_memory": "'bool', when true, it assumes the dataset is in memory, i.e., input_fn should return the entire dataset as a single batch, n_batches_per_layer should be set as 1, num_worker_replicas should be 1, and num_ps_replicas should be 0 in 'tf.Estimator.RunConfig'. Raises: ValueError: when wrong arguments are given or unsupported functionalities are requested."
    },
    "BoostedTreesRegressor": {
        "feature_columns": "An iterable containing all the feature columns used by the model. All items in the set should be instances of classes derived from 'FeatureColumn'.",
        "n_batches_per_layer": "the number of batches to collect statistics per layer. The total number of batches is total number of data divided by batch size.",
        "model_dir": "Directory to save model parameters, graph and etc. This can also be used to load checkpoints from the directory into a estimator to continue training a previously saved model.",
        "label_dimension": "Number of regression targets per example. Multi-dimensional support is not yet implemented.",
        "weight_column": "A string or a 'NumericColumn' created by 'tf.fc_old.numeric_column' defining feature column representing weights. It is used to downweight or boost examples during training. It will be multiplied by the loss of the example. If it is a string, it is used as a key to fetch weight tensor from the 'features'. If it is a 'NumericColumn', raw tensor is fetched by key 'weight_column.key', then weight_column.normalizer_fn is applied on it to get weight tensor.",
        "n_trees": "number trees to be created.",
        "max_depth": "maximum depth of the tree to grow.",
        "learning_rate": "shrinkage parameter to be used when a tree added to the model.",
        "l1_regularization": "regularization multiplier applied to the absolute weights of the tree leafs.",
        "l2_regularization": "regularization multiplier applied to the square weights of the tree leafs.",
        "tree_complexity": "regularization factor to penalize trees with more leaves.",
        "min_node_weight": "min_node_weight: minimum hessian a node must have for a split to be considered. The value will be compared with sum(leaf_hessian)/(batch_size * n_batches_per_layer).",
        "config": "'RunConfig' object to configure the runtime settings.",
        "center_bias": "Whether bias centering needs to occur. Bias centering refers to the first node in the very first tree returning the prediction that is aligned with the original labels distribution. For example, for regression problems, the first node will return the mean of the labels. For binary classification problems, it will return a logit for a prior probability of label 1.",
        "pruning_mode": "one of 'none', 'pre', 'post' to indicate no pruning, pre- pruning (do not split a node if not enough gain is observed) and post pruning (build the tree up to a max depth and then prune branches with negative gain). For pre and post pruning, you MUST provide tree_complexity >0.",
        "quantile_sketch_epsilon": "float between 0 and 1. Error bound for quantile computation. This is only used for float feature columns, and the number of buckets generated per float feature is 1/quantile_sketch_epsilon.",
        "train_in_memory": "'bool', when true, it assumes the dataset is in memory, i.e., input_fn should return the entire dataset as a single batch, n_batches_per_layer should be set as 1, num_worker_replicas should be 1, and num_ps_replicas should be 0 in 'tf.Estimator.RunConfig'. Raises: ValueError: when wrong arguments are given or unsupported functionalities are requested."
    },
    "DNNLinearCombinedClassifier": {
        "model_dir": "Directory to save model parameters, graph and etc. This can also be used to load checkpoints from the directory into a estimator to continue training a previously saved model.",
        "linear_feature_columns": "An iterable containing all the feature columns used by linear part of the model. All items in the set must be instances of classes derived from 'FeatureColumn'.",
        "linear_optimizer": "An instance of 'tf.Optimizer' used to apply gradients to the linear part of the model. Can also be a string (one of 'Adagrad', 'Adam', 'Ftrl', 'RMSProp', 'SGD'), or callable. Defaults to FTRL optimizer.",
        "dnn_feature_columns": "An iterable containing all the feature columns used by deep part of the model. All items in the set must be instances of classes derived from 'FeatureColumn'.",
        "dnn_optimizer": "An instance of 'tf.Optimizer' used to apply gradients to the deep part of the model. Can also be a string (one of 'Adagrad', 'Adam', 'Ftrl', 'RMSProp', 'SGD'), or callable. Defaults to Adagrad optimizer.",
        "dnn_hidden_units": "List of hidden units per layer. All layers are fully connected.",
        "dnn_activation_fn": "Activation function applied to each layer. If None, will use 'tf.nn.relu'.",
        "dnn_dropout": "When not None, the probability we will drop out a given coordinate.",
        "n_classes": "Number of label classes. Defaults to 2, namely binary classification. Must be > 1.",
        "weight_column": "A string or a '_NumericColumn' created by 'tf.feature_column.numeric_column' defining feature column representing weights. It is used to down weight or boost examples during training. It will be multiplied by the loss of the example. If it is a string, it is used as a key to fetch weight tensor from the 'features'. If it is a '_NumericColumn', raw tensor is fetched by key 'weight_column.key', then weight_column.normalizer_fn is applied on it to get weight tensor.",
        "label_vocabulary": "A list of strings represents possible label values. If given, labels must be string type and have any value in 'label_vocabulary'. If it is not given, that means labels are already encoded as integer or float within [0, 1] for 'n_classes=2' and encoded as integer values in {0, 1,..., n_classes-1} for 'n_classes'>2 . Also there will be errors if vocabulary is not provided and labels are string.",
        "config": "RunConfig object to configure the runtime settings.",
        "warm_start_from": "A string filepath to a checkpoint to warm-start from, or a 'WarmStartSettings' object to fully configure warm-starting. If the string filepath is provided instead of a 'WarmStartSettings', then all weights are warm-started, and it is assumed that vocabularies and Tensor names are unchanged.",
        "loss_reduction": "One of 'tf.losses.Reduction' except 'NONE'. Describes how to reduce training loss over batch. Defaults to 'SUM_OVER_BATCH_SIZE'.",
        "batch_norm": "Whether to use batch normalization after each hidden layer.",
        "linear_sparse_combiner": "A string specifying how to reduce the linear model if a categorical column is multivalent. One of \"mean\", \"sqrtn\", and \"sum\" -- these are effectively different ways to do example-level normalization, which can be useful for bag-of-words features. For more details, see 'tf.feature_column.linear_model'. Raises: ValueError: If both linear_feature_columns and dnn_features_columns are empty at the same time."
    },
    "DNNLinearCombinedRegressor": {
        "model_dir": "Directory to save model parameters, graph and etc. This can also be used to load checkpoints from the directory into a estimator to continue training a previously saved model.",
        "linear_feature_columns": "An iterable containing all the feature columns used by linear part of the model. All items in the set must be instances of classes derived from 'FeatureColumn'.",
        "linear_optimizer": "An instance of 'tf.Optimizer' used to apply gradients to the linear part of the model. Can also be a string (one of 'Adagrad', 'Adam', 'Ftrl', 'RMSProp', 'SGD'), or callable. Defaults to FTRL optimizer.",
        "dnn_feature_columns": "An iterable containing all the feature columns used by deep part of the model. All items in the set must be instances of classes derived from 'FeatureColumn'.",
        "dnn_optimizer": "An instance of 'tf.Optimizer' used to apply gradients to the deep part of the model. Can also be a string (one of 'Adagrad', 'Adam', 'Ftrl', 'RMSProp', 'SGD'), or callable. Defaults to Adagrad optimizer.",
        "dnn_hidden_units": "List of hidden units per layer. All layers are fully connected.",
        "dnn_activation_fn": "Activation function applied to each layer. If None, will use 'tf.nn.relu'.",
        "dnn_dropout": "When not None, the probability we will drop out a given coordinate.",
        "label_dimension": "Number of regression targets per example. This is the size of the last dimension of the labels and logits 'Tensor' objects (typically, these have shape '[batch_size, label_dimension]').",
        "weight_column": "A string or a '_NumericColumn' created by 'tf.feature_column.numeric_column' defining feature column representing weights. It is used to down weight or boost examples during training. It will be multiplied by the loss of the example. If it is a string, it is used as a key to fetch weight tensor from the 'features'. If it is a '_NumericColumn', raw tensor is fetched by key 'weight_column.key', then weight_column.normalizer_fn is applied on it to get weight tensor.",
        "config": "RunConfig object to configure the runtime settings.",
        "warm_start_from": "A string filepath to a checkpoint to warm-start from, or a 'WarmStartSettings' object to fully configure warm-starting. If the string filepath is provided instead of a 'WarmStartSettings', then all weights are warm-started, and it is assumed that vocabularies and Tensor names are unchanged.",
        "loss_reduction": "One of 'tf.losses.Reduction' except 'NONE'. Describes how to reduce training loss over batch. Defaults to 'SUM_OVER_BATCH_SIZE'.",
        "batch_norm": "Whether to use batch normalization after each hidden layer.",
        "linear_sparse_combiner": "A string specifying how to reduce the linear model if a categorical column is multivalent. One of \"mean\", \"sqrtn\", and \"sum\" -- these are effectively different ways to do example-level normalization, which can be useful for bag-of-words features. For more details, see 'tf.feature_column.linear_model'. Raises: ValueError: If both linear_feature_columns and dnn_features_columns are empty at the same time."
    },
    "sqlflow_models.DNNClassifier": {
        "feature_columns": "feature columns. :type feature_columns: list[tf.feature_column].",
        "hidden_units": "number of hidden units. :type hidden_units: list[int].",
        "n_classes": "List of hidden units per layer. :type n_classes: int."
    },
    "sqlflow_models.DeepEmbeddingClusterModel": {
        "feature_columns": "a list of tf.feature_column",
		"n_clusters": "Number of clusters.",
        "kmeans_init": "Number of running K-Means to get best choice of centroids.",
        "run_pretrain": "Run pre-train process or not.",
        "existed_pretrain_model": "Path of existed pre-train model. Not used now.",
        "pretrain_dims": "Dims of layers which is used for build autoencoder.",
        "pretrain_activation_func": "Active function of autoencoder layers.",
        "pretrain_batch_size": "Size of batch when pre-train.",
        "train_batch_size": "Size of batch when run train.",
        "pretrain_epochs": "Number of epochs when pre-train.",
        "pretrain_initializer": "Initialize function for autoencoder layers.",
        "train_max_iters": "Number of iterations when train.",
        "update_interval": "Interval between updating target distribution.",
        "tol": "tol.",
        "loss": "Default 'kld' when init."
    },
    "sqlflow_models.StackedBiLSTMClassifier": {
        "feature_columns": "All columns must be embedding of sequence column with same sequence_length. :type feature_columns: list[tf.embedding_column].",
        "stack_units": "Units for LSTM layer. :type stack_units: vector of ints.",
        "n_classes": "Target number of classes. :type n_classes: int."
    },
    "xgboost.gbtree": {
        "max_depth": "int Maximum tree depth for base learners.",
        "learning_rate": "float Boosting learning rate (xgb's \"eta\")",
        "n_estimators": "int Number of trees to fit.",
        "verbosity": "int The degree of verbosity. Valid values are 0 (silent) - 3 (debug).",
        "silent": "boolean Whether to print messages while running boosting. Deprecated. Use verbosity instead.",
        "objective": "string or callable Specify the learning task and the corresponding learning objective or a custom objective function to be used (see note below).",
        "nthread": "int Number of parallel threads used to run xgboost. (Deprecated, please use ''n_jobs'')",
        "n_jobs": "int Number of parallel threads used to run xgboost. (replaces ''nthread'')",
        "gamma": "float Minimum loss reduction required to make a further partition on a leaf node of the tree.",
        "min_child_weight": "int Minimum sum of instance weight(hessian) needed in a child.",
        "max_delta_step": "int Maximum delta step we allow each tree's weight estimation to be.",
        "subsample": "float Subsample ratio of the training instance.",
        "colsample_bytree": "float Subsample ratio of columns when constructing each tree.",
        "colsample_bylevel": "float Subsample ratio of columns for each level.",
        "colsample_bynode": "float Subsample ratio of columns for each split.",
        "reg_alpha": "float (xgb's alpha) L1 regularization term on weights",
        "reg_lambda": "float (xgb's lambda) L2 regularization term on weights",
        "scale_pos_weight": "float Balancing of positive and negative weights.",
        "base_score": "The initial prediction score of all instances, global bias.",
        "seed": "int Random number seed. (Deprecated, please use random_state)",
        "random_state": "int Random number seed. (replaces seed)",
        "missing": "float, optional Value in the data which needs to be present as a missing value. If None, defaults to np.nan.",
        "importance_type": "string, default \"gain\" The feature importance type for the feature_importances_ property: either \"gain\", \"weight\", \"cover\", \"total_gain\" or \"total_cover\"."
    },
    "xgboost.gblinear": {
        "max_depth": "int Maximum tree depth for base learners.",
        "learning_rate": "float Boosting learning rate (xgb's \"eta\")",
        "n_estimators": "int Number of trees to fit.",
        "verbosity": "int The degree of verbosity. Valid values are 0 (silent) - 3 (debug).",
        "silent": "boolean Whether to print messages while running boosting. Deprecated. Use verbosity instead.",
        "objective": "string or callable Specify the learning task and the corresponding learning objective or a custom objective function to be used (see note below).",
        "nthread": "int Number of parallel threads used to run xgboost. (Deprecated, please use ''n_jobs'')",
        "n_jobs": "int Number of parallel threads used to run xgboost. (replaces ''nthread'')",
        "gamma": "float Minimum loss reduction required to make a further partition on a leaf node of the tree.",
        "min_child_weight": "int Minimum sum of instance weight(hessian) needed in a child.",
        "max_delta_step": "int Maximum delta step we allow each tree's weight estimation to be.",
        "subsample": "float Subsample ratio of the training instance.",
        "colsample_bytree": "float Subsample ratio of columns when constructing each tree.",
        "colsample_bylevel": "float Subsample ratio of columns for each level.",
        "colsample_bynode": "float Subsample ratio of columns for each split.",
        "reg_alpha": "float (xgb's alpha) L1 regularization term on weights",
        "reg_lambda": "float (xgb's lambda) L2 regularization term on weights",
        "scale_pos_weight": "float Balancing of positive and negative weights.",
        "base_score": "The initial prediction score of all instances, global bias.",
        "seed": "int Random number seed. (Deprecated, please use random_state)",
        "random_state": "int Random number seed. (replaces seed)",
        "missing": "float, optional Value in the data which needs to be present as a missing value. If None, defaults to np.nan.",
        "importance_type": "string, default \"gain\" The feature importance type for the feature_importances_ property: either \"gain\", \"weight\", \"cover\", \"total_gain\" or \"total_cover\"."
    },
    "xgboost.dart": {
        "max_depth": "int Maximum tree depth for base learners.",
        "learning_rate": "float Boosting learning rate (xgb's \"eta\")",
        "n_estimators": "int Number of trees to fit.",
        "verbosity": "int The degree of verbosity. Valid values are 0 (silent) - 3 (debug).",
        "silent": "boolean Whether to print messages while running boosting. Deprecated. Use verbosity instead.",
        "objective": "string or callable Specify the learning task and the corresponding learning objective or a custom objective function to be used (see note below).",
        "nthread": "int Number of parallel threads used to run xgboost. (Deprecated, please use ''n_jobs'')",
        "n_jobs": "int Number of parallel threads used to run xgboost. (replaces ''nthread'')",
        "gamma": "float Minimum loss reduction required to make a further partition on a leaf node of the tree.",
        "min_child_weight": "int Minimum sum of instance weight(hessian) needed in a child.",
        "max_delta_step": "int Maximum delta step we allow each tree's weight estimation to be.",
        "subsample": "float Subsample ratio of the training instance.",
        "colsample_bytree": "float Subsample ratio of columns when constructing each tree.",
        "colsample_bylevel": "float Subsample ratio of columns for each level.",
        "colsample_bynode": "float Subsample ratio of columns for each split.",
        "reg_alpha": "float (xgb's alpha) L1 regularization term on weights",
        "reg_lambda": "float (xgb's lambda) L2 regularization term on weights",
        "scale_pos_weight": "float Balancing of positive and negative weights.",
        "base_score": "The initial prediction score of all instances, global bias.",
        "seed": "int Random number seed. (Deprecated, please use random_state)",
        "random_state": "int Random number seed. (replaces seed)",
        "missing": "float, optional Value in the data which needs to be present as a missing value. If None, defaults to np.nan.",
        "importance_type": "string, default \"gain\" The feature importance type for the feature_importances_ property: either \"gain\", \"weight\", \"cover\", \"total_gain\" or \"total_cover\"."
    }
}
`
