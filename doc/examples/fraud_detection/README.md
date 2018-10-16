# Credit Card Fraud Detection

Data collected from [here](https://www.kaggle.com/mlg-ulb/creditcardfraud)

To run: `python premade_estimator.py`, which gives
```
Adding numeric_columns: Time
Adding numeric_columns: V1
Adding numeric_columns: V2
Adding numeric_columns: V3
Adding numeric_columns: V4
Adding numeric_columns: V5
Adding numeric_columns: V6
Adding numeric_columns: V7
Adding numeric_columns: V8
Adding numeric_columns: V9
Adding numeric_columns: V10
Adding numeric_columns: V11
Adding numeric_columns: V12
Adding numeric_columns: V13
Adding numeric_columns: V14
Adding numeric_columns: V15
Adding numeric_columns: V16
Adding numeric_columns: V17
Adding numeric_columns: V18
Adding numeric_columns: V19
Adding numeric_columns: V20
Adding numeric_columns: V21
Adding numeric_columns: V22
Adding numeric_columns: V23
Adding numeric_columns: V24
Adding numeric_columns: V25
Adding numeric_columns: V26
Adding numeric_columns: V27
Adding numeric_columns: V28
Adding numeric_columns: Amount
INFO:tensorflow:Using default config.
WARNING:tensorflow:Using temporary folder as model directory: /tmp/tmpzj6rzutf
INFO:tensorflow:Using config: {'_num_worker_replicas': 1, '_global_id_in_cluster': 0, '_save_checkpoints_steps': None, '_tf_random_seed': None, '_evaluation_master': '', '_protocol': None, '_session_config': allow_soft_placement: true
graph_options {
  rewrite_options {
    meta_optimizer_iterations: ONE
  }
}
, '_model_dir': '/tmp/tmpzj6rzutf', '_device_fn': None, '_cluster_spec': <tensorflow.python.training.server_lib.ClusterSpec object at 0x7f40f9b40438>, '_save_summary_steps': 100, '_eval_distribute': None, '_keep_checkpoint_max': 5, '_master': '', '_train_distribute': None, '_log_step_count_steps': 100, '_num_ps_replicas': 0, '_experimental_distribute': None, '_save_checkpoints_secs': 600, '_service': None, '_task_type': 'worker', '_task_id': 0, '_keep_checkpoint_every_n_hours': 10000, '_is_chief': True}
INFO:tensorflow:Calling model_fn.
INFO:tensorflow:Done calling model_fn.
INFO:tensorflow:Create CheckpointSaverHook.
INFO:tensorflow:Graph was finalized.
INFO:tensorflow:Running local_init_op.
INFO:tensorflow:Done running local_init_op.
INFO:tensorflow:Saving checkpoints for 0 into /tmp/tmpzj6rzutf/model.ckpt.
INFO:tensorflow:loss = 892.3532, step = 1
INFO:tensorflow:global_step/sec: 295.764
INFO:tensorflow:loss = 0.00025717804, step = 101 (0.338 sec)
INFO:tensorflow:global_step/sec: 446.454
INFO:tensorflow:loss = 0.00024299514, step = 201 (0.224 sec)
INFO:tensorflow:global_step/sec: 444.479
INFO:tensorflow:loss = 0.00020988214, step = 301 (0.225 sec)
INFO:tensorflow:global_step/sec: 421.979
INFO:tensorflow:loss = 0.00023327809, step = 401 (0.237 sec)
INFO:tensorflow:global_step/sec: 446.172
INFO:tensorflow:loss = 0.00020405005, step = 501 (0.224 sec)
INFO:tensorflow:global_step/sec: 431.785
INFO:tensorflow:loss = 0.00017921966, step = 601 (0.232 sec)
INFO:tensorflow:global_step/sec: 442.912
INFO:tensorflow:loss = 0.00018488085, step = 701 (0.226 sec)
INFO:tensorflow:global_step/sec: 444.628
INFO:tensorflow:loss = 0.00017795907, step = 801 (0.225 sec)
INFO:tensorflow:global_step/sec: 428.385
INFO:tensorflow:loss = 0.00016896412, step = 901 (0.233 sec)
INFO:tensorflow:Saving checkpoints for 1000 into /tmp/tmpzj6rzutf/model.ckpt.
INFO:tensorflow:Loss for final step: 0.00016340206.
INFO:tensorflow:Calling model_fn.
WARNING:tensorflow:Trapezoidal rule is known to produce incorrect PR-AUCs; please switch to "careful_interpolation" instead.
WARNING:tensorflow:Trapezoidal rule is known to produce incorrect PR-AUCs; please switch to "careful_interpolation" instead.
INFO:tensorflow:Done calling model_fn.
INFO:tensorflow:Starting evaluation at 2018-10-16-01:02:49
INFO:tensorflow:Graph was finalized.
INFO:tensorflow:Restoring parameters from /tmp/tmpzj6rzutf/model.ckpt-1000
INFO:tensorflow:Running local_init_op.
INFO:tensorflow:Done running local_init_op.
INFO:tensorflow:Finished evaluation at 2018-10-16-01:02:49
INFO:tensorflow:Saving dict for global step 1000: accuracy = 1.0, accuracy_baseline = 1.0, auc = 0.9999999, auc_precision_recall = 0.0, average_loss = 0.0001784083, global_step = 1000, label/mean = 0.0, loss = 0.0016056747, precision = 0.0, prediction/mean = 0.00017826515, recall = 0.0
INFO:tensorflow:Saving 'checkpoint_path' summary for global step 1000: /tmp/tmpzj6rzutf/model.ckpt-1000

Test set accuracy: 1.00000

INFO:tensorflow:Calling model_fn.
INFO:tensorflow:Done calling model_fn.
INFO:tensorflow:Graph was finalized.
INFO:tensorflow:Restoring parameters from /tmp/tmpzj6rzutf/model.ckpt-1000
INFO:tensorflow:Running local_init_op.
INFO:tensorflow:Done running local_init_op.

Prediction is "Not Fraud" (100.0%), expected "Not Fraud"

Prediction is "Not Fraud" (100.0%), expected "Not Fraud"

Prediction is "Not Fraud" (100.0%), expected "Not Fraud"
```
