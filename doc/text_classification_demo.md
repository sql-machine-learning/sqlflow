# Training a Text Classification Model Using SQLFlow

This is a tutorial on how to train a Text Classification Model Using SQLFlow.
Note that the steps in this tutorial may be changed during the development
of SQLFlow, we only provide a way that simply works for the current version.

To support custom models like CNN text classification, you may check out the
current [design](https://github.com/sql-machine-learning/models/blob/develop/doc/customized%2Bmodel.md)
for ongoing development.

In this tutorial we use two datasets both for english and chinese text classification.
The case using chinese dataset is more complicated since Chinese sentences can not be
segmented by spaces. You can download the full dataset from:

1. [IMDB-Movie-Reviews-Dataset](https://www.kaggle.com/iarunava/imdb-movie-reviews-dataset)
1. [chinese-text-classification-dataset](https://github.com/fate233/toutiao-text-classfication-dataset)

# Steps to Process and Train With IMDB Dataset

1. Download full IMDB dataset from the above link and unzip the content.
1. Use [this](https://gist.github.com/typhoonzero/45c8097648152adfc4f6aef772a05e0a)
   script to load data into MySQL database and do preprocess like segmentation,
   map to word id, and padding. You can also modify the script's MySQL connection
   address to your own MySQL installation.
1. Then use the following statements to train and predict using SQLFlow:
    ```sql
    SELECT *
    FROM imdb.train_processed
    TRAIN DNNClassifier
    WITH
    n_classes = 2,
    hidden_units = [512, 128]
    COLUMN content
    LABEL class
    INTO sqlflow_models.my_text_model_en;

    SELECT *
    FROM imdb.test_processed
    PREDICT imdb.predict.class
    USING sqlflow_models.my_text_model_en;
    ```
1. Then you can get predict result from table `imdb.predict`:

# Steps to Run Chinese Text Classification Dataset

1. Download the dataset from the above link and unpack `toutiao_cat_data.txt.zip`.
1. Copy `toutiao_cat_data.txt` to `/var/lib/mysql-files/` on the server your MySQL located on, this is
   because MySQL may prevent importing data from an untrusted location.
1. Login to MySQL command line like `mysql -uroot -p` and create a database and table to load the
   dataset, note the table must create with `CHARSET=utf8 COLLATE=utf8_unicode_ci` so that the Chinese
   texts can be correctly shown.
    ```sql
    CREATE DATABASE toutiao;
    CREATE TABLE `train` (
        `id` bigint(20) NOT NULL,
        `class_id` int(3) NOT NULL,
        `class_name` varchar(100) NOT NULL,
        `news_title` varchar(255) NOT NULL,
        `news_keywords` varchar(255) NOT NULL)
    ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;

    CREATE TABLE `train_processed` (
        `id` bigint(20) NOT NULL,
        `class_id` int(3) NOT NULL,
        `class_name` varchar(100) NOT NULL,
        `news_title` TEXT NOT NULL,
        `news_keywords` varchar(255) NOT NULL)
    ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;

    CREATE TABLE `test_processed` (
        `id` bigint(20) NOT NULL,
        `class_id` int(3) NOT NULL,
        `class_name` varchar(100) NOT NULL,
        `news_title` TEXT NOT NULL,
        `news_keywords` varchar(255) NOT NULL)
    ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;
    COMMIT;
    ```
1. In the MySQL shell, type below line to load the dataset into created table:
    ```sql
    LOAD DATA LOCAL
    INFILE '/var/lib/mysql-files/toutiao_cat_data.txt'
    INTO TABLE train
    CHARACTER SET utf8
    FIELDS TERMINATED by '_!_'
    LINES TERMINATED by "\n";
    ```
1. Run [this](https://gist.github.com/typhoonzero/dd3d814f3d4bae4538842df2a659d278)
   python script to generate a vocabulary, and process the raw news title texts to padded word ids. The max length of the segmented sentence is `92`. Note that this python script also change the `class_id`
   column's value to `0~17` which originally is `100~117` since we accept label start from `0`.
1. Split some of the data into a validation table, and remove the validation
   data from train data:
    ```sql
    INSERT INTO `test_processed` (`id`, `class_id`, `class_name`, `news_title`, `news_keywords`)
    SELECT `id`, `class_id`, `class_name`, `news_title`, `news_keywords` FROM `train_processed`
    ORDER BY RAND()
    LIMIT 5000;

    DELETE FROM `train_processed` WHERE id IN (
        SELECT id FROM `test_processed` AS p
    )
    ```
1. Then use the following statements to train and predict using SQLFlow:
    ```sql
    SELECT *
    FROM toutiao.train_processed
    TRAIN DNNClassifier
    WITH
    n_classes = 17,
    hidden_units = [128, 512]
    COLUMN news_title
    LABEL class_id
    INTO sqlflow_models.my_text_model;

    SELECT *
    FROM toutiao.test_processed
    PREDICT toutiao.predict.class_id
    USING sqlflow_models.my_text_model;
    ```
1. Then you can get predict result from table `toutiao.predict`:
