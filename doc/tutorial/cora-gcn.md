# Classify Cora Dataset Using GCN

This tutorial shows how to train [GCN](https://arxiv.org/pdf/1609.02907.pdf) model on the Cora dataset. In this tutorial, you will learn how to:
- Train a GCN on the [Cora citation dataset](https://linqs-data.soe.ucsc.edu/public/lbc/cora.tgz).
- Use the trained GCN to predict the label for some of the papers in the dataset.


## The Dataset

The Cora dataset is graph dataset about a citation network of scientific papers. It consists of 2708 scientific publications classified into one of seven classes. The citation network consists of 5429 links. Each publication in the dataset is described by a 0/1-valued word vector indicating the absence/presence of the corresponding word from the dictionary. The dictionary consists of 1433 unique words.

Due to the fact that graph data is not applicable to be divided into batches, we are not able to split the data and store in training dataset and test dataset respectively. Thus, we represent the entire graph through two tables: `Node Table` and `Edge Table`. (Anyone who wants to store graph data in the database can refer to this method.)

Here are the column description of the `Node Table`:

Column | Explain 
-- | -- 
id| Id for the node.
name| Name for the node.
features| Feature vector of the node represented in the string. An example would be "0,0,1".
label | Label for the node.

The following command can be used to construct the `Node Table`. 

```sql
CREATE TABLE cora.node (
        id INT,
        node_name TEXT,
        features  TEXT,
        label TEXT);
```

Here are the column description of the `Edge Table`:

Column | Explain 
-- | -- 
id| Id for the edge.
from_node_id| Id for the from node of the edge.
to_node_id| Id for the to node of the edge.
weight | Weight for the edge.

The following command can be used to construct the `Edge Table`. 


```sql
CREATE TABLE cora.edge (
        id INT,
        from_node_id INT,
        to_node_id  INT,
        weight FLOAT);
```

You can have a quick peek of the data by running the following standard SQL statements.

```sql
%%sqlflow
describe cora.node;
describe cora.edge;
```

```sql
%%sqlflow
select * from cora.node limit 10;
```

## Train GCN on the Cora dataset

Let's train a GCN model!

### Load data from Cora
You can load the data from database following a standard SQL command such as `SELECT * FROM cora.node`. However, since GCN is supposed to deal with graph data, you have to load both the `Node Table` and `Edge Table` at once.

In order to do so, you need to use the `JOIN` command in SQL to select all the data from `Node Table` and `Edge Table`. The following command is used to load all the data for training GCN.

```sql
%%sqlflow
SELECT cora.node.id, features, label as class, cora.edge.from_node_id, cora.edge.to_node_id FROM cora.node
LEFT JOIN cora.edge ON (cora.node.id = cora.edge.from_node_id OR cora.node.id = cora.edge.to_node_id)
ORDER BY cora.node.id
```
The `OR` statement in the command is used to select all the possible bidirectional edges from the dataset. Without this `OR` command, some of the edges will be missing and it is not possible to construct the entire graph.

With the `COLUMN` clause provided, SQLFlow can handle the comma separated string `features` with command `COLUMN DENSE(features)`.

The GCN in SQLFlow is able to build the entire graph automatically with input in the folloing order: `node.id`, `node.features`, `node.label`, `edge.from_node_id`, `edge.to_node_id`. Please make sure the order is correct in order to run GCN successfully.

### Train GCN
After loading the dataset, you would be able to train GCN with following command:

```sql
%%sqlflow
TO TRAIN sqlflow_models.GCN
WITH model.nhid=16, 
     model.nclass=7, 
     model.epochs=200, 
     model.train_ratio=0.15, 
     model.eval_ratio=0.2, 
     validation.metrics="CategoricalAccuracy"
```

You can specify the model parameters and training configurations through the `WITH` clause. For instance, you could set the `epochs` to be trained to be 100. `train_ratio` and `eval_ratio` indicate the proportion of the dataset to used for training and evaluate respectively. You can also change configurations such as `nlayer` which decides the number `GCNLayer` to be used, `dropout` which defines the dropout rate of the model.

Combing with the data loading commands, you can start to train the GCN model using:

```sql
%%sqlflow
SELECT cora.node.id, features, label as class, cora.edge.from_node_id, cora.edge.to_node_id FROM cora.node
LEFT JOIN cora.edge ON (cora.node.id = cora.edge.from_node_id OR cora.node.id = cora.edge.to_node_id)
ORDER BY cora.node.id
TO TRAIN sqlflow_models.GCN
WITH model.nhid=16, model.nclass=7, 
     model.epochs=200, model.train_ratio=0.15, 
     model.eval_ratio=0.2, validation.metrics="CategoricalAccuracy"
COLUMN DENSE(features)
LABEL class
INTO sqlflow_models.gcn_model;
```

The details of the training will be outputed in the following format:

```
Epoch 100 loss=0.455858 accuracy=0.943350 val_acc=0.857934
```

**ATTENTION**: if you store the data in the database with different column names for `id`, `features`, `from_node_id` and `to_node_id`, you need to specify the name through `WITH` command in order to let the model get the data successfully:

```sql
WITH model.id_col='id', -- string to be the name for id of each node
     model.feature_col='features', -- ... name for feature column of each node
     model.from_node_col='from_node_id', -- ... name for from_node_id of each edge
     model.to_node_col='to_node_id' -- ... name for to_node_id of each edge
```

## Predict the label of a paper in Cora dataset

To specify the prediction data, we use standard SQL statements like `SELECT id FROM cora.node LIMIT 5`.

The pretrained GCN model is previously stored at `sqlflow_models.gcn_model`. You could get the prediction data and write the predicted result into table `cora.predict` column `class`. Note that GCN only supports prediction using node's `id` because all the prediction is already complete during training phase and results are stored regarding to node's id. You can write the following SQLFlow prediction statement:

```sql
%%sqlflow
SELECT id FROM cora.node TO PREDICT cora.predict.class USING sqlflow_models.gcn_model;
```

After the prediction, you can check the prediction result by

```sql
%%sqlflow
SELECT * FROM cora.predict LIMIT 5;
```

## Evaluate GCN

With the support of SQLFlow, you can evaluate the model's performance on the evaluation dataset. GCN will generate the evaluation results during training phase, so one can get the evaluation result with:

```sql
%%sqlflow
SELECT cora.node.id, features, label as class, cora.edge.from_node_id, cora.edge.to_node_id FROM cora.node
LEFT JOIN cora.edge ON (cora.node.id = cora.edge.from_node_id OR cora.node.id = cora.edge.to_node_id)
ORDER BY cora.node.id
WITH model.nhid=16, model.nclass=7, 
     model.epochs=200, model.train_ratio=0.15, 
     model.eval_ratio=0.2, validation.metrics="CategoricalAccuracy"
COLUMN DENSE(features)
TO EVALUATE sqlflow_models.gcn_model
INTO gcn_evaluation;
```
* `gcn_evaluation` is the result table that stores the evaluation results.