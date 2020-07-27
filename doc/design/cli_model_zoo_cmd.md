# SQLFlow Command-line Support Model Zoo Commands

## Background

As described in [model zoo design doc](contribute_models_new.md), developers can write their own models and publish them to the `Model Zoo` which is a service dedicated to manage [model definitions](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/model_zoo.md#concepts) and [models](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/model_zoo.md#concepts). `Model Zoo` is designed in server-client mode. Multi developers can access the same `Model Zoo` server through their own clients. `sqlflow` is a command-line tool currently used to access SQLFlow server. We plan to extend the functionalities of this tool and make it the client of `Model Zoo` server.

## Overall Design
From the user's perspective, the command can be divided into categories like access `SQLFlow server` and operate the `Model Zoo`. So, we can take a classic sub-command format like below, which is written in [docopt](http://docopt.org/) syntax:
```bash
SQLFlow Command-line Tool.

Usage:
    sqlflow [options] run [-d <data_source> -e <program> -f <file>]
    sqlflow [options] release repo [--force] <model_dir> <name_version>
    sqlflow [options] release model [--force] <model> <version>
    sqlflow [options] delete repo <name_version>
    sqlflow [options] delete model <model> <version>
    sqlflow [options] list repo
    sqlflow [options] list model

Options:
    -v, --version                   print the version and exit
    -h, --help                      print this screen
    -c, --cert-file=<file>          cert file to connect SQLFlow or Model Zoo server
    --env-file=<file>               config file in KEY=VAL format
    -s, --sqlflow-server=<addr>     SQLFlow server address and port
    -m, --model-zoo-server=<addr>   Model Zoo server address and port
    -u, --user=<user>               Model Zoo user account
    -p, --password=<password>       Model Zoo user password

Run Options:
    -d, --data-source=<data_source>   data source to operate
    -e, --execute=<program>           execute given program
    -f, --file=<file>                 execute program in file

Release Options:
    --force                  force overwrite existing model

```
## Implementation

### Command-line Parsing
As the command-line is written in `docopt`, it can be parsed by existing parsers like [docopt.go](https://github.com/docopt/docopt.go). After that, we can easily get all the sub commands and their params in the command line.

### Model Uploading
For model definitions, we can simply tar the whole directory and upload them through the [gRPC interface](https://github.com/sql-machine-learning/sqlflow/blob/14d6a28be13418bec8a17091a0db22b5c76a1fc2/pkg/proto/modelzooserver.proto#L91). For models, there already exists [some code](https://github.com/sql-machine-learning/sqlflow/blob/14d6a28be13418bec8a17091a0db22b5c76a1fc2/pkg/model/model.go#L77) to export the model from database to file system. We can upload them after the exporting.

## Model and Repo Listing
SQLFlow command-line tool support listing released repos/models.  By default, users can only list the repos/models released by himself.  So, we need to add authentication info in the listing requests.  We added `--user` and the `--password` options to handle this.  As there may be a lot of models and repos, the implementation will pull the list for multiple times, each time for just a small number of results.

## Action Plan
We will implement the core logic of the command-line, which is the uploading and deleting of objects in the `Model Zoo`.
SQLFlow command-line tool may need some authentication process for further operation. This may be implemented by username/password or by certification file. Also, some of the params in the command-line can be written into env file, we postpone the implementation of these features.