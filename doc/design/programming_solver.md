# Mathematical Programming Solver with SQLFlow

Mathematical programming (aka. [mathematical optimization](https://en.wikipedia.org/wiki/Mathematical_optimization)) is the selection of a best element (with regard to some criterion) from some set of available alternatives. Solving optimization problems is widely used in fields like economics and finance, computer science, engineering, and researching. In this design, we try to make SQLFlow be able to solve below branches of programming problems using SQL statements by leveraging systems like [pyomo](http://www.pyomo.org/) or our internal programming project at Ant Group, they both support these types of programming categories:

- [Linear programming](https://en.wikipedia.org/wiki/Linear_programming)
- [Quadratic programming](https://en.wikipedia.org/wiki/Quadratic_programming)
- [Nonlinear programming](https://en.wikipedia.org/wiki/Nonlinear_programming)
- [Integer programming](https://en.wikipedia.org/wiki/Integer_programming)
- [Mixed-integer linear programming](http://macc.mcmaster.ca/maccfiles/chachuatnotes/07-MILP-I_handout.pdf)
- [Mixed-integer quadratic programming](http://www.optimization-online.org/DB_FILE/2014/07/4446.pdf)
- [Mixed-integer nonlinear programming](https://link.springer.com/book/10.1007/978-1-4614-1927-3)

## Background

To understand what's a mathematical programming problem, let's take a look at this example (example originally published at http://faculty.kutztown.edu/vasko/MAT121/MAT121web/Example_2.html):

Giapetto’s Woodcarving, Inc., manufactures two types of wooden toys: soldiers and trains. 

A soldier sells for $27 and uses $10 worth of raw materials.  Each soldier that is manufactured increases Giapetto’s variable labor and overhead costs by $14.  A train sells for $21 and uses $9 worth of raw materials.  Each train built increases Giapetto’s variable labor and overhead costs by $10.  The manufacture of wooden soldiers and trains requires two types of skilled labor: carpentry and finishing.  A soldier requires 2 hours of finishing labor and 1 hour of carpentry labor.  A train requires 1 hour of finishing labor and 1 hour of carpentry labor.  Each week, Giapetto can obtain all the needed raw material but only 100 finishing hours and 80 carpentry hours.  Demand for trains is unlimited, but at most 40 soldiers are bought each week.  Giapetto wants to maximize weekly profit (revenues-costs).

Let

- x be the number of soldiers produced each week
- y be the number of trains produced each week

Then the objective is: 

**Maximise Z = (27 - 10 - 14)x + (21 - 9 - 10)y = 3x + 2y**

The constraints are:

- 2x + y <= 100 (Finishing Constraint)
- x + y <= 80 (Carpentry Constraint)
- x <=40 (Constraint on demand for soldiers)
- x,y >= 0 (sign restriction)

To solve this problem, we can use tools like [AMPL](https://en.wikipedia.org/wiki/AMPL) or open-source tools like [pyomo](http://www.pyomo.org/), or [Matlab](https://www.mathworks.com/help/optim/ug/linprog.html), or [R](https://towardsdatascience.com/linear-programming-in-r-444e9c199280).

You can learn from the examples in the above links that:

- Using Matlab and R are quite the same, they require users to define the constraints as matrixes and call a function to get the result. They both have their own grammar, and you have to write code according to their language specifications.
- Using "AMPL like" high-level language to describe the problem, and call the corresponding solvers. Pyomo and AMPL have similar components in their grammar: sets, parameters, variables, objectives, constraints (https://pyomo.readthedocs.io/en/stable/pyomo_modeling_components/index.html).

So we can see that using AMPL is a modern and general way of describing mathematical programming problems. We can simply write the AMPL snippet to describe the above problem:

```
set X;
var ProductAmt{x in X} >= 0;

param Price{x in X};
param MaterialCost{x in X};
param OtherCost{x in X};
param Finishing{x in X};
param Carpentry{x in X};
param Demand{x in X};

maximize profit:
    sum{x in X} (Price[x] - MaterialCost[x] - OtherCost[x]) * ProductAmt[x];

s.t. finishing: sum{x in X} Finishing[x] * ProductAmt[x] <= 100;
s.t. carpentry: sum{x in X} Carpentry[x] * ProductAmt[x] <= 80;
s.t. demand{x in X}: ProductAmt[x] <= 40;
```

## Grammar Design

In order to extend SQL to have completely same ability of AMPL, the extended syntax should be able to describe **objective and constraints** while the input data table can store the **params** for each **variable**, and the rows in the table is naturally become the **set** we defined in AMPL. 

### Linear Programming Syntax

Then we have the below table `woodcarving`:

| product | price | materials_cost | other_cost | finishing | carpentry | max_num |
| ------- | ----- | -------------- | ---------- | --------- | --------- | ------- |
| soldier | 27    | 10             | 14         | 2         | 1         | 40      |
| train   | 21    | 9              | 10         | 1         | 1         | 10000   |

In the `woodcarving`:

- The set X is row one and row two.
- We have one variable, and the variable name strings is stored in column `product`. In cases that have cross variables (like the example described at https://en.wikipedia.org/wiki/AMPL), the table should have multiple string columns to store the variable names.
- Other columns like `price`, `materials_cost` are all params for the corresponding variable.

Then we can use below extended SQL syntax to describe above example:

```sql
SELECT * FROM woodcarving
TO MAXIMIZE SUM((price - materials_cost - other_cost) * amount)
CONSTRAINT SUM(finishing * amount) <= 100,
           SUM(carpentry * amount) <= 80,
           amount <= max_num
WITH variable="amount(product)",
     var_type="Integers"
[USING glpk]
INTO result_table;
```

In the SQL statement:

- `TO MAXIMIZE|MINIMIZE ...` defines an expression string that describes the objective. 
    - The syntax `MAXIMIZE|MINIMIZE` is used to specify the objective sense. 
    - In the expression, `SUM` means sum the value across all rows like normal SQL statements.
- `CONSTRAINT ...` expression strings that describe the constraints, can have multiple `CONSTRAINT` expressions separated by comma.
- `WITH` attributes:
    - `variable="amount(product)"`: **required**, specify the variable definition, `product` is the column that stores the variable name. Using comma to separate if there are multiple variables, e.g. `shipment(plants,markets)`.
    - `var_type="Integers"`: **optional**, specify the variable type, there should only be one variable in current cases. The format is like `var_type="Type"`,  the type can be `Integers`, `NonNegativeIntegers`, `Reals` etc. The default variable type is `Integers`.
- `USING`: **optional**, solver tool to use, default: glpk.
- `INTO result_table`: set the result table name.

After the SQL statement finishes execution, the result table `result_table` should look like:

| product | amount |
| ------  | ------ |
| soldier | 20     |
| train   | 60     |

### Combinatorial Optimization Syntax

Combinatorial Optimization Problem (https://en.wikipedia.org/wiki/Combinatorial_optimization) is a subset of mathematical optimization that is widely used in real life. Here we demostrate how to use a SQL statement to solve a typicall combinational optimization problem.

For example, there are several plants that manufactures products and several markets that sells them (see the example described in https://en.wikipedia.org/wiki/AMPL for details), we want to minimize the cost of transportation between plants and markets, we have three tables looks like below:

1. Plants capacity table:

    | plants  | capacity |
    | ------- | -------- |
    | plantA  | 100      |
    | plantB  | 90       |

2. Markets demand table:

    | markets |  demand |
    | ------- | ------- |
    | marketA | 130     |
    | marketB | 60      |

3. Plants to markets distance table:

    | plants  | markets | distance |
    | ------- | ------- | -------- |
    | plantA  | marketA |  140     |
    | plantA  | marketB |  210     |
    | plantB  | marketA |  300     |
    | plantB  | marketB |  90      |

4. When we start to solve the problem, we'd like to join the tables beforehand:

    ```sql
    SELECT trans.plants, trans.markets, trans.distance, plants.capacity, markets.demand FROM transportation AS trans
    LEFT JOIN plants ON src.plants = plants.plants
    LEFT JOIN markets ON src.markets = markets.markets;
    ```
    Then we have a "joined" table like below to start the solving process:

    | plants  | markets | distance | capacity | demand |
    | ------- | ------- | -------- | -------- | ------ |
    | plantA  | marketA |  140     | 100      | 130    |
    | plantA  | marketB |  210     | 100      | 60     |
    | plantB  | marketA |  300     | 90       | 130    |
    | plantB  | marketB |  90      | 90       | 60     |

Then we can use below extended SQL syntax to describe above example:

```sql
SELECT src.plants, src.markets, src.distance, plants.capacity, markets.demand FROM transportation AS src
LEFT JOIN plants ON src.plants = plants.plants
LEFT JOIN markets ON src.markets = markets.markets
TO MINIMIZE SUM(shipment * distance * 90 / 1000)
CONSTRAINT SUM(shipment) <= capacity GROUP BY plants,
           SUM(shipment) >= demand GROUP BY markets
WITH variable="shipment(plants,markets)",
     var_type="Integers"
[USING glpk]
INTO result_table;
```

- In the above SQL statement, the syntax is quite the same as the single variable example, yet
- The `CONSTRAINT` including a `GROUP BY` clause is a "partial aggregation constraint", take `CONSTRAINT SUM(markets) <= capacity GROUP BY plants` as an example, it means:
    1. for each plant,
    2. the sum of "shipment amount to each market from current plant" should be less than the current plant's capacity.

Then after the solving job has completed, we should have below contents in the `result_table` (the result column is a fake result for demonstration):

| plants  | markets | shipment |
| ------- | ------- | -------- |
| plantA  | marketA |  123     |
| plantA  | marketB |  123     |
| plantB  | marketA |  123     |
| plantB  | marketB |  123     |


### Aggregation Functions

Support any aggregation functions accross rows that the programming solvers support. We may need to add support more syntax than `SUM` in the future.

## Implementation

1. Update our extended syntax parser to support `TO MAXIMIZE|MINIMIZE` clauses.
1. Add an IR struct to represent `TO MAXIMIZE|MINIMIZE` clause.
1. Create a table to store the result.
1. Add code generator to generate code like below example to run, for different mathematical programming software, we may need to add different code generators. Since we extend SQL to have the same ability that AMPL has, we can almost connect to any software we like.
1. The generated code should be able to output the result to the result table.


## Intermediate Representation

The extended `TO MAXIMIZE|MINIMIZE` syntax can be represented by below Go structure after parsing:

```go
type SolveExpr struct {
    // Expression parsed from SQL statement of objective and constraints, used for code generation.
    // e.g. sum((@TABLE.price[i] - @TABLE.materials_cost[i] - @TABLE.other_cost[i]) * @X[i])
    Expression string
    // Iterate variables like {i in X} in the above example.
    IterVars []string{}
}

type MathProgrammingStmt struct {
    // Select is the select statement before TO MAXIMIZE|MINIMIZE clause.
    Select string
    // Attributes is a map of parsed attribute in the WITH clause.
    Attributes map[string]interface{}
    // Objective
    Objective SolveExpr
    // ObjectiveSense, 0: maximize, 1: minimize
    ObjectiveSense int
    // Constraints
    Constraints []*SolveExpr{}
    // ResultTable is the table name to store results.
    ResultTable string
    // When SQLFLOW_submitter == "pai", tmp tables will be created for solving tasks
    TmpTrainTable    string
}
```

## Example of Generated Code


```python
from pyomo.opt import SolverFactory
from pyomo.environ import (ConcreteModel, Var, Objective, maximize, Constraint, NonNegativeIntegers)

model = ConcreteModel()

# input dataframe
data_df = ... # Construct dataframe from DBMS driver here.

# variable
size = len(data_df.type)
variables = range(size)
model.x = Var(variables, within=NonNegativeIntegers)

# object
def objRule(model):
    return sum([(data_df.price[i] - data_df.materials_cost[i] - data_df.other_cost[i]) * model.x[i] for i in model.x])
model.Obj = Objective(rule=objRule, sense=maximize)

# constrains
def rule1(model):
    return sum([data_df.finishing[i] * model.x[i] for i in model.x]) <= 100
model.c1 = Constraint(rule=rule1)

def rule2(model):
    return sum([data_df.carpentry[i] * model.x[i] for i in model.x]) <= 80
model.c2 = Constraint(rule=rule2)

def rule3(model):
    return model.x[i] <= data_df.max_num[i] for i in model.x
model.c3 = Constraint(rule=rule3)

if __name__ == '__main__':
    with SolverFactory('glpk', executable='glpsol') as solver:
        results = solver.solve(model)
        print(results)

        model.display()
    
    result_data = pd.DataFrame(columns=['var_id', 'var', 'x'])
    result_data['var_id'] = [i for i in model.x]
    result_data['var'] = data_df.type.values
    result_data['x'] = [model.x[i]() for i in model.x]
```
