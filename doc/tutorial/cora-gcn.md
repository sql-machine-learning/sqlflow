# Classify Cora Dataset Using GCN

This tutorial shows how to train a [GCN](https://arxiv.org/pdf/1609.02907.pdf) model on the Cora dataset. In this tutorial, you will learn how to:
- Train a GCN model on the [Cora citation dataset](https://linqs-data.soe.ucsc.edu/public/lbc/cora.tgz).
- Use the trained GCN model to predict the label for some of the papers in the dataset.


## The Dataset

The Cora dataset is a graph dataset about a citation network of scientific papers. It consists of 2708 scientific publications classified into one of seven classes. The citation network consists of 5429 links. Each publication in the dataset is described by a 0/1-valued word vector indicating the absence/presence of the corresponding word from the dictionary. The dictionary consists of 1433 unique words.

Due to the fact that graph data is not applicable to be divided into batches, we are not able to split the data and store in the training dataset and test dataset respectively. Thus, we represent the entire graph through two tables: `Node Table` and `Edge Table`. (Anyone who wants to store graph data in the database can refer to this method.)

Here are the column description of the `Node Table`:

Column | Explain | Type
-- | -- | --
id| Id for the node. | Integer
name| Name for the node. | Text
features| Feature vector of the node represented in the `csv` format. An example would be "0,0,1". |  Text
label | Label for the node. |  Text

The following command can be used to construct the `Node Table`. 

```text
CREATE TABLE cora.node (
        id INT,
        node_name TEXT,
        features  TEXT,
        label TEXT);
```

Here are the column description of the `Edge Table`:

Column | Explain | Type
-- | -- | --
id| Id for the edge. | Integer
from_node_id| Id for the from node of the edge. | Integer
to_node_id| Id for the to node of the edge. | Integer
weight | Weight for the edge. | Float

The following command can be used to construct the `Edge Table`. 


```text
CREATE TABLE cora.edge (
        id INT,
        from_node_id INT,
        to_node_id  INT,
        weight FLOAT);
```

You can have a quick peek of the data by running the following standard SQL statements.

```sql
%%sqlflow
DESCRIBE cora.node;
DESCRIBE cora.edge;
```

```sql
%%sqlflow
SELECT * FROM cora.node LIMIT 10;
```

## Train the GCN on the Cora dataset

Let's train a GCN model!

### Load data from Cora
You can load the data from the database following a standard SQL command such as `SELECT * FROM cora.node`. However, since the GCN model is supposed to deal with the graph data, you have to load both the `Node Table` and `Edge Table` at once.

In order to do so, you need to use the `JOIN` command in SQL to select all the data from `Node Table` and `Edge Table`. The following command is used to load all the data for training GCN.

```sql
%%sqlflow
SELECT cora.node.id, features, label as class, cora.edge.from_node_id, cora.edge.to_node_id FROM cora.node
LEFT JOIN cora.edge ON (cora.node.id = cora.edge.from_node_id OR cora.node.id = cora.edge.to_node_id)
ORDER BY cora.node.id;
```

The `OR` statement in the command is used to select all the possible bidirectional edges from the dataset. Without this `OR` statement, some of the edges will be missing and it is not possible to construct the entire graph.

With the `COLUMN` clause provided, SQLFlow can handle the comma separated string `features` with command `COLUMN DENSE(features)`.

The GCN model in SQLFlow is able to build the entire graph automatically with inputs in the folloing order: `node.id`, `node.features`, `node.label`, `edge.from_node_id`, `edge.to_node_id`. Please make sure the order is correct in order to run the GCN model successfully.

### Train GCN

Here is the table that lists all the parameters of the GCN model:

Parameter | Description | Type
-- | -- | --
nhid | Number of hidden units for GCN. | Integer
nclass | Number of classes in total which will be the output dimension. | Integer
epochs | Number of epochs for the model to be trained. | Integer
train_ratio | Percentage of data to be used for training. | Float
eval_ratio | Percentage of data points to be used for evaluating. | Float
early_stopping | Whether to use early stopping trick during the training phase. | Boolean
dropout | The rate for dropout. | Float
nlayer | Number of GCNLayer to be used in the model. | Integer
id_col | Name for the column in database to be used as the id of each node. | String
feature_col | Name for the column in database to be used as the features of each node. | String
from_node_col | Name for the column in database to be used as the from_node id of each edge. | String
to_node_col | Name for the column in database to be used as the to_node id of each edge. | String

After loading the dataset, you would be able to train the GCN model with following command:

```text
TO TRAIN sqlflow_models.GCN
WITH model.nhid=16, 
     model.nclass=7, 
     model.epochs=200, 
     model.train_ratio=0.15, 
     model.eval_ratio=0.2, 
     validation.metrics="CategoricalAccuracy"
```

You can specify the model parameters and training configurations through the `WITH` clause. For instance, you could set the `model.epochs` to be trained to be 100. `model.train_ratio` and `model.eval_ratio` indicate the proportion of the dataset to used for training and evaluate respectively. You can also change configurations such as `model.nlayer` which decides the number `GCNLayer` to be used, and `model.dropout` which defines the dropout rate of the model. (For more parameters, please refer to the table above.)

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

```text
Epoch 100 loss=0.455858 accuracy=0.943350 val_acc=0.857934
```

**ATTENTION**: if you store the data in the database with different column names for `id`, `features`, `from_node_id` and `to_node_id`, you need to specify the name through `WITH` command in order to let the model get the data successfully:

```text
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

## Evaluate the Trained GCN Model

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