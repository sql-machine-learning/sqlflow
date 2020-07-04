// Copyright 2020 The SQLFlow Authors. All rights reserved.
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

package ast

// Constraint represents a constraint in the description of a mathematical
// optimization problem.  A constraint is an expression with an optional GroupBy
// name.
type Constraint struct {
	*Expr
	GroupBy string
}

// ConstraintList is a list of constraints.  The parser generates a
// ConstraintList from the TO MAXIMIZE/MINIMIZE clause.
type ConstraintList []*Constraint
