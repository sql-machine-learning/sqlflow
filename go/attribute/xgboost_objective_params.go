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

package attribute

// XGBoostObjectiveDocs is xgboost objective parameter docs extracted
// from https://xgboost.readthedocs.io/en/latest/parameter.html
var XGBoostObjectiveDocs = map[string]string{
	"binary:hinge":        "hinge loss for binary classification. This makes predictions of 0 or 1, rather than producing probabilities.",
	"binary:logistic":     "logistic regression for binary classification, output probability",
	"binary:logitraw":     "logistic regression for binary classification, output score before logistic transformation",
	"multi:softmax":       "set XGBoost to do multiclass classification using the softmax objective, you also need to set num_class(number of classes)",
	"multi:softprob":      "same as softmax, but output a vector of ndata * nclass, which can be further reshaped to ndata * nclass matrix. The result contains predicted probability of each data point belonging to each class.",
	"rank:map":            "Use LambdaMART to perform list-wise ranking where Mean Average Precision (MAP) is maximized",
	"rank:ndcg":           "Use LambdaMART to perform list-wise ranking where Normalized Discounted Cumulative Gain (NDCG) is maximized",
	"rank:pairwise":       "Use LambdaMART to perform pairwise ranking where the pairwise loss is minimized",
	"reg:gamma":           "gamma regression with log-link. Output is a mean of gamma distribution. It might be useful, e.g., for modeling insurance claims severity, or for any outcome that might be gamma-distributed.",
	"reg:linear":          "linear regression",
	"reg:logistic":        "logistic regression",
	"reg:squarederror":    "regression with squared loss.",
	"reg:squaredlogerror": "regression with squared log loss 1/2[log(pred+1)\u2212log(label+1)]^2",
	"reg:tweedie":         "Tweedie regression with log-link. It might be useful, e.g., for modeling total loss in insurance, or for any outcome that might be Tweedie-distributed.",
	"survival:cox":        "Cox regression for right censored survival time data (negative values are considered right censored). Note that predictions are returned on the hazard ratio scale (i.e., as HR = exp(marginal_prediction) in the proportional hazard function h(t) = h0(t) * HR).",
}
