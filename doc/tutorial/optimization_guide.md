# Use SQLFlow to Solve Optimization Problems

<a href="https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/optimization_guide.ipynb&fileName=sqlflow_tutorial_optimization.ipynb">
  <img alt="Open In PAI-DSW" src="https://pai-public-data.oss-cn-beijing.aliyuncs.com/EN-pai-dsw.svg">
</a>

This document explains how to use the SQLFlow to solve the optimization problems.

## The Optimization SQL Syntax

The optimization SQL syntax in SQLFlow is as follows:

```sql
SELECT ... FROM ...
TO MAXIMIZE|MINIMIZE
    objective_expression
CONSTRAINT 
    constraint_rule_1 [GROUP BY column_1],
    constraint_rule_2 [GROUP BY column_2],
    ...
    constraint_rule_n [GROUP BY column_name_n]
WITH
    variables = "result_value_name(column_1,column_2,...,column_m)",
    var_type = "Binary"|"Integers"|"Reals"|...
[USING glpk]
INTO output_database.output_table;
```

where:

- `SELECT ... FROM ...`: any standard SQL query statement.
- `TO MAXIMIZE|MINIMIZE`: whether to maximize or minimize the value of the `objective_expression`.
- `objective_expression` : the objective expression to be maximized or minimized.
- `CONSTRAINT`:  indicates the constraint rules. We should separate each constraint rule using the comma. There may be `GROUP BY` clauses at the end of some constraint rules. For example, `GROUP BY column_1` means that we would apply the constraint rule to each unique cell value of the column `column_1`.
- `variables`: indicates the variable names and value to be optimized, where `result_value_name` means the variable value to be optimized,  and the string values in the brackets, i.e. `column_1,column_2,...,column_m` are the column names of the variables to be optimized.
- `var_type`: the domain of the variable value. SQLFlow supports the following `var_type`:
    - `Binary`: the variable value can be only 0 or 1.
    - `Integers`: the variable value can be only integers.
    - `PositiveIntegers`, `NegativeIntegers`: the variable value can be only positive or negative integers.
    - `NonPositiveIntegers`, `NonNegativeIntegers`: the variable value can be only non-positive or non-negative integers.
    - `Reals`: the variable value can be only real numbers.
    - `PositiveReals`, `NegativeReals`: the variable value can be positive or negative real numbers.
    - `NonPositiveReals`, `NonNegativeReals`: the variable value can be non-positive or non-negative real numbers.
- `USING glpk`: indicates the solver to solve the problem. Currently, only `glpk` is supported. Please see [here](https://www.gnu.org/software/glpk/) for details on the GLPK solver.  The `USING` clause will be optional if we use the `glpk` solver.
- `INTO ...`: indicates the output table to save the solved result.

## Example 1: Single Column Case

Let us take an example to explain how to use SQLFlow to solve optimization problems. You can refer to this case for details [here](http://faculty.kutztown.edu/vasko/MAT121/MAT121web/Example_2.html).

Giapetto’s Woodcarving, Inc., manufactures two types of wooden toys: soldiers and trains. A soldier sells for 27 dollar and uses 10 dollar worth of raw materials.  Each soldier that is manufactured increases Giapetto’s variable labor and overhead costs by 14 dollar.  A train sells for 21 dollar and uses 9 dollar worth of raw materials.  Each train built increases Giapetto’s variable labor and overhead costs by 10 dollar.  The manufacture of wooden soldiers and trains requires two types of skilled labor: carpentry and finishing.  A soldier requires 2 hours of finishing labor and 1 hour of carpentry labor.  A train requires 1 hour of finishing labor and 1 hour of carpentry labor.  Each week, Giapetto can obtain all the needed raw material but only 100 finishing hours and 80 carpentry hours.  At most 10000 trains and at most 40 soldiers are bought each week.  Giapetto wants to maximize weekly profit (revenues-costs).

Let

- x be the number of soldiers produced each week
- y be the number of trains produced each week

Then the objective is: 

**Maximize Z = (27 - 10 - 14)x + (21 - 9 - 10)y**

The constraints are:

- 2*x + 1*y <= 100 (finishing constraint)
- 1*x + 1*y <= 80 (carpentry constraint)
- x <= 40, y <= 10000 (demand constraint)
- both x,y are non-negative integers. 

The table `my_db.woodcarving` corresponding to the example above is:

| product | price | materials_cost | other_cost | finishing | carpentry | max_num |
| ------- | ----- | -------------- | ---------- | --------- | --------- | ------- |
| soldier | 27    | 10             | 14         | 2         | 1         | 40      |
| train   | 21    | 9              | 10         | 1         | 1         | 10000   |

We can create the data table `my_db.woodcarving` using the following SQL statements:

```sql
%%sqlflow
CREATE DATABASE IF NOT EXISTS my_db;

DROP TABLE IF EXISTS my_db.woodcarving;

CREATE TABLE my_db.woodcarving (
    product VARCHAR(255),
    price FLOAT,
    materials_cost FLOAT,
    other_cost FLOAT,
    finishing FLOAT,
    carpentry FLOAT,
    max_num FLOAT
);

INSERT INTO my_db.woodcarving VALUES('soldier', 27, 10, 14, 2, 1, 40);
INSERT INTO my_db.woodcarving VALUES('train', 21, 9, 10, 1, 1, 10000);
```

The SQLFlow optimization SQL statement for this case would be:

```sql
%%sqlflow
SELECT * FROM my_db.woodcarving -- the input data source
TO MAXIMIZE 
    SUM((price - materials_cost - other_cost) * amount) -- the objective expression
CONSTRAINT 
    SUM(finishing * amount) <= 100, -- finishing constraint, i.e, 2*x + 1*y <= 100
    SUM(carpentry * amount) <= 80, -- carpentry constraint, i.e., 1*x + 1*y <= 80
    amount <= max_num -- demand constraint, i.e., x <= 40, y <= 10000
WITH 
    variables="amount(product)", -- amount = (x, y) is the value to be optimized, product is the column name of the variable
    var_type="NonNegativeIntegers" -- amount = (x, y) is inside the domain of non-negative integers
USING glpk -- use the GLPK solver to solve the linear optimization problem
INTO my_db.woodcarving_result_table;
```

Once the SQLFlow server receives the SQL statement above, it would call the GLPK solver to solve the optimization problem described in the SQL statement. After solving the problem, we would get the following logs:

```text
Solved result is:

   product  amount
   
0  soldier      20

1    train      60

Saved in my_db.woodcarving_result_table.

Objective value is 180.0
```

We can also examine the solved result by the SQL statement:

```sql
%%sqlflow
SELECT * FROM my_db.woodcarving_result_table;
```

|product|amount|
|---    |---   |
|soldier| 20   |
|train  | 60   |


## Example 2: Multiple Columns Case with GROUP BY

Suppose that there are several plants that manufacture products, and several markets that sell them (see the example described [here](https://en.wikipedia.org/wiki/AMPL) for details). We want to minimize the cost of transportation between plants and markets.

We have three tables that look like below:

1. Plants capacity table `my_db.plants`, where the column `capacity` indicates the maximum product number that each plant can manufacture. The product number should be integers.

| plants  | capacity |
|---      |---       |
| plantA  | 100      |
| plantB  | 90       |

2. Markets demand table `my_db.markets`, where the column `demand` indicates the required product number of each market.

| markets |  demand |
|---      |---      |
| marketA | 130     |
| marketB | 60      |

3. Plants to markets distance table `my_db.transportation`, where the column `distance` is the distance to transport each plant to each market.

| plants  | markets | distance |
|---      |---      |---       |
| plantA  | marketA |  140     |
| plantA  | marketB |  210     |
| plantB  | marketA |  300     |
| plantB  | marketB |  90      |

We can create the tables above using the following SQL statements:
```sql
%%sqlflow
CREATE DATABASE IF NOT EXISTS my_db;

DROP TABLE IF EXISTS my_db.plants;
CREATE TABLE my_db.plants (
    plants VARCHAR(255),
    capacity FLOAT
);
INSERT INTO my_db.plants VALUES('plantA', 100), ('plantB', 90);

DROP TABLE IF EXISTS my_db.markets;
CREATE TABLE my_db.markets (
    markets VARCHAR(255),
    demand FLOAT
);
INSERT INTO my_db.markets VALUES('marketA', 130), ('marketB', 60);

DROP TABLE IF EXISTS my_db.transportation;
CREATE TABLE my_db.transportation (
    plants VARCHAR(255),
    markets VARCHAR(255),
    distance FLOAT
);
INSERT INTO my_db.transportation VALUES('plantA', 'marketA', 140);
INSERT INTO my_db.transportation VALUES('plantA', 'marketB', 210);
INSERT INTO my_db.transportation VALUES('plantB', 'marketA', 300);
INSERT INTO my_db.transportation VALUES('plantB', 'marketB', 90);
```

When we start to solve the problem, we would like to join the tables beforehand:

```sql
%%sqlflow
SELECT 
    t.plants AS plants, 
    t.markets AS markets, 
    t.distance AS distance, 
    p.capacity AS capacity, 
    m.demand AS demand
FROM my_db.transportation AS t
LEFT JOIN my_db.plants AS p ON t.plants = p.plants
LEFT JOIN my_db.markets AS m ON t.markets = m.markets;
```

Then we have a "joined" table like below to start the solving process:

| plants  | markets | distance | capacity | demand |
| ------- | ------- | -------- | -------- | ------ |
| plantA  | marketA |  140     | 100      | 130    |
| plantB  | marketA |  300     | 90       | 130    |
| plantA  | marketB |  210     | 100      | 60     |
| plantB  | marketB |  90      | 90       | 60     |


Then we can use below extended SQL syntax to describe the above example:

```sql
%%sqlflow
SELECT 
    t.plants AS plants, 
    t.markets AS markets, 
    t.distance AS distance, 
    p.capacity AS capacity, 
    m.demand AS demand
FROM my_db.transportation AS t
LEFT JOIN my_db.plants AS p ON t.plants = p.plants
LEFT JOIN my_db.markets AS m ON t.markets = m.markets  -- the joined SQL statement
TO MINIMIZE 
    SUM(amount * distance) -- the objective expression, minimize the total transportation distance
CONSTRAINT 
    SUM(amount) <= capacity GROUP BY plants,
    SUM(amount) >= demand GROUP BY markets 
WITH 
    -- amount is the value to be optimized, "plants" and "markets" are the column names of the variables
    variables="amount(plants,markets)",
    var_type="NonNegativeIntegers" -- the amount value should be non-negative integers
USING glpk
INTO my_db.transportation_result_table;
```

where there are `GROUP BY` clauses in the constraint rules, which mean:

- `SUM(amount) <= capacity GROUP BY plant` : for each plant, the sum of the amount value should not exceed the capacity of the plant.
- `SUM(amount) >= demand GROUP BY markets` : for each market, the sum of the amount value should be larger than or equal to the demand of the market.

After solving the problem, we would get the following logs:

```
Solved result is:

   plants  markets  amount

0  plantA  marketA     100

1  plantB  marketA      30

2  plantA  marketB       0

3  plantB  marketB      60

Saved in my_db.transportation_result_table.

Objective value is 28400.0
```

We can also examine the solved result by the SQL statement:

```sql
%%sqlflow
SELECT * FROM my_db.transportation_result_table;
```

| plants | markets | amount |
|---     |---      |---     |
| plantA | marketA |    100 |
| plantB | marketA |     30 |
| plantA | marketB |      0 |
| plantB | marketB |     60 |

## Summary
In the above examples, we explain how to use the SQLFlow to solve the optimization problems. Currently, we only support the linear optimization problem and the GLPK solver. We would support more optimization problems and solvers in the future version.
