# The Entity Relationship Diagram of Model Market

This is an early draft of the idea of Model Market.  It should be merged into a formal design later.

![Alt text](https://g.gravizo.com/source/model_market_er?https%3A%2F%2Fraw.githubusercontent.com%2Fsql-machine-learning%2Fsqlflow%2Fmodel_market_er%2Fdoc%2Fdesign%2Fmodel_market_er.md)

<details> 
<summary></summary>
model_market_er
  digraph G {
	node [fontname="Arial"];	
	account [shape=record, label="account|{username|password|profile}"];
	organization [shape=record, label="organization|{name|description}"];
	model_def [shape=record, label="model\ndefinition|{Git repo|Docker image}"];
	model_def_version [shape=record, label="model\ndefinition\nversion|{Docker image tag}"];
	trained_model [shape=record, label="trained\nmodel|{training data|hyperparameters}"];
	account -> organization [label="has\n1:1-N"];
	organization -> model_def [label="contains\n1:0-N"];
	model_def -> model_def_version [label="has\n1:0-N"];
	model_def_version -> trained_model [label="trained into\n1:0-N"];
  }
model_market_er
</details>
