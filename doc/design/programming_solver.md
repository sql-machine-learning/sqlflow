# Mathematical Programming Solver with SQLFlow

Mathematical programming (aka. [mathematical optimization](https://en.wikipedia.org/wiki/Mathematical_optimization)) is the selection of a best element (with regard to some criterion) from some set of available alternatives. Solving optimization problems is widely used in fields like economics and finance, computer science, engineering, and researching. In this design, we try to make SQLFlow be able to solve below branches of programming problems using SQL statements by leveraging systems like [pyomo](http://www.pyomo.org/):

- [Linear programming](https://en.wikipedia.org/wiki/Linear_programming)
- [Quadratic programming](https://en.wikipedia.org/wiki/Quadratic_programming)
- [Nonlinear programming](https://en.wikipedia.org/wiki/Nonlinear_programming)
- [Integer programming](https://en.wikipedia.org/wiki/Integer_programming)
- [Mixed-integer linear programming](http://macc.mcmaster.ca/maccfiles/chachuatnotes/07-MILP-I_handout.pdf)
- [Mixed-integer quadratic programming](http://www.optimization-online.org/DB_FILE/2014/07/4446.pdf)
- [Mixed-integer nonlinear programming](https://link.springer.com/book/10.1007/978-1-4614-1927-3)

## Background

To understand what's a mathematical programming problem, let's take a look at this example (example origially published at http://faculty.kutztown.edu/vasko/MAT121/MAT121web/Example_2.html):

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


To solve this problem, we can write objective and constraints using SQL, then SQLFlow should call the corresponding "solver" (e.g. [pyomo](http://www.pyomo.org/) and [GLPK](https://www.gnu.org/software/glpk/)) to find the result and output the result into the result table.

## Grammar Design

Generally, SQLFlow users can write one SQL statement to describe the programming problem:

```sql
SELECT * FROM train_table
TO SOLVE LP
WITH
     objective="sum([(train_table.price[i] - train_table.materials_cost[i] - train_table.other_cost[i]) * @X[i] for i in @X])",
     constraints=["sum([train_table.finishing[i] * @X[i] for i in @X]) <= 100",
                  "sum([train_table.carpentry[i] * @X[i] for i in @X]) <= 80",
                  "@X[i] <= train_table.max_num[i] for i in @X",
                  "@X[i] >= 0 for i in @X"],
     var_name_col="type",
     var_type="Integers",
     solver="glpk"
INTO result_table;
```

The `train_table` looks like:

|  type   | price | materials_cost | other_cost | finishing | carpentry | max_num |
| ------- | ----- | -------------- | ---------- | --------- | --------- | ------- |
| soldier | 27    | 10             | 14         | 2         | 1         | 40      |
| train   | 21    | 9              | 10         | 1         | 1         | 10000   |

Note that we create this table to store the variables to form the objective and constraints so that when we have hundreds of variable types (e.g. the company actually manufactures 1000 types of products), the SQL statement will keep the same.

In the SQL statement:

- `TO SOLVE LP`: set to use the "Linear Programming Solver". The notation `LP` means "Linear Programming Solver", and we can have other solvers like `QP` for "Quadratic programming Solver" etc.
- `INTO result_table`: set the result table name.
- `WITH` attributes:
    - objective: **required**, an expression string that describes the objective. Notation `@X` will be replaced to input dataframe when generating Python code.
    - constraints: **required**, a list of expression strings that describe the constraints.
    - var_name_col: **required**, specify one column that stores the variable name.
    - var_type: **required**, specify the variable type, can be `Integers`, `NonNegativeIntegers`, `Reals` etc.
    - solver: *optional*, solver tool to use, default: glpk.

After the SQL statement finishes execution, the result table `result_table` should look like:

| type    | result |
| ------  | ------ |
| soldier | 20     |
| train   | 60     |

## Implementation

1. Update our extended syntax parser to support `TO SOLVE` clause.
1. Add an IR struct to represent `TO SOLVE` clause.
1. Create a table to store the result.
1. Add code generator to generate code like below example to run.
1. The generated code should be able to output the result to the result table.


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
