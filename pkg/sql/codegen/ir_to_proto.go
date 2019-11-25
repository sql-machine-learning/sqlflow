// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

package codegen

import (
	"fmt"
	"strings"

	pb "sqlflow.org/sqlflow/pkg/server/proto"
	irpb "sqlflow.org/sqlflow/pkg/sql/codegen/proto"
)

// FIXME(typhoonzero): copied from tensorflow/codegen.go
func intArrayToJSONString(ia []int) string {
	return strings.Join(strings.Split(fmt.Sprint(ia), " "), ",")
}

// FIXME(typhoonzero): copied from tensorflow/codegen.go
func attrToPythonValue(attr interface{}) string {
	switch attr.(type) {
	case int:
		return fmt.Sprintf("%d", attr.(int))
	case int64:
		return fmt.Sprintf("%d", attr.(int64))
	case float32:
		return fmt.Sprintf("%f", attr.(float32))
	case float64: // FIXME(typhoonzero): may never use
		return fmt.Sprintf("%f", attr.(float64))
	case []int:
		return intArrayToJSONString(attr.([]int))
		// TODO(typhoonzero): support []float etc.
	case []interface{}:
		tmplist := attr.([]interface{})
		if len(tmplist) > 0 {
			if _, ok := tmplist[0].(int); ok {
				intlist := []int{}
				for _, v := range tmplist {
					intlist = append(intlist, v.(int))
				}
				return intArrayToJSONString(intlist)
			}
		}
		// TODO(typhoonzero): support []float etc.
		return "[]"
	case string:
		return attr.(string)
	default:
		return ""
	}
}

// FIXME(typhoonzero): copied from tensorflow/codegen.go
func dtypeToString(dt FieldType) string {
	switch dt {
	case Float:
		return "float32"
	case Int:
		return "int64"
	case String:
		return "string"
	default:
		return ""
	}
}

func toInt32List(il []int) []int32 {
	ret := []int32{}
	for _, i := range il {
		ret = append(ret, int32(i))
	}
	return ret
}

func fieldMetaToPbMeta(fm *FieldMeta) *irpb.FieldMeta {
	return &irpb.FieldMeta{
		Name:       fm.Name,
		Dtype:      dtypeToString(fm.DType),
		Delimiter:  fm.Delimiter,
		Shape:      toInt32List(fm.Shape),
		IsSparse:   fm.IsSparse,
		Vocabulary: fm.Vocabulary,
		MaxID:      int32(fm.MaxID),
	}
}

func featureColumnToPb(fc FeatureColumn) (*irpb.FeatureColumn, error) {
	switch fc.(type) {
	case *NumericColumn:
		nc := &irpb.FeatureColumn{
			FeatureColumn: &irpb.FeatureColumn_Nc{
				Nc: &irpb.NumericColumn{
					FieldMeta: fieldMetaToPbMeta(fc.GetFieldMeta()[0]),
				},
			},
		}
		return nc, nil
	case *BucketColumn:
		fm := fc.GetFieldMeta()[0]
		bc := &irpb.FeatureColumn{
			FeatureColumn: &irpb.FeatureColumn_Bc{
				Bc: &irpb.BucketColumn{
					SourceColumn: &irpb.NumericColumn{
						FieldMeta: fieldMetaToPbMeta(fm),
					},
					Boundaries: toInt32List(fc.(*BucketColumn).Boundaries),
				},
			},
		}
		return bc, nil
	case *CrossColumn:
		cc := fc.(*CrossColumn)
		pbkeys := []*irpb.FeatureColumn{}
		for _, key := range cc.Keys {
			tmpfc, err := featureColumnToPb(key.(FeatureColumn))
			if err != nil {
				return nil, err
			}
			pbkeys = append(pbkeys, tmpfc)
		}
		pbcc := &irpb.FeatureColumn{
			FeatureColumn: &irpb.FeatureColumn_Cc{
				Cc: &irpb.CrossColumn{
					Keys:           pbkeys,
					HashBucketSize: int32(cc.HashBucketSize),
				},
			},
		}
		return pbcc, nil
	case *CategoryIDColumn:
		catc := fc.(*CategoryIDColumn)
		pbcatc := &irpb.FeatureColumn{
			FeatureColumn: &irpb.FeatureColumn_Catc{
				Catc: &irpb.CategoryIDColumn{
					FieldMeta:  fieldMetaToPbMeta(fc.GetFieldMeta()[0]),
					BucketSize: int32(catc.BucketSize),
				},
			},
		}
		return pbcatc, nil
	case *SeqCategoryIDColumn:
		seqcatc := fc.(*SeqCategoryIDColumn)
		pbseqcatc := &irpb.FeatureColumn{
			FeatureColumn: &irpb.FeatureColumn_Seqcatc{
				Seqcatc: &irpb.SeqCategoryIDColumn{
					FieldMeta:  fieldMetaToPbMeta(fc.GetFieldMeta()[0]),
					BucketSize: int32(seqcatc.BucketSize),
				},
			},
		}
		return pbseqcatc, nil
	case *EmbeddingColumn:
		emb := fc.(*EmbeddingColumn)
		tmpfc, err := featureColumnToPb(emb.CategoryColumn.(FeatureColumn))
		if err != nil {
			return nil, err
		}
		_, iscatc := emb.CategoryColumn.(*CategoryIDColumn)
		if iscatc {
			embcatc := &irpb.EmbeddingColumn_CategoryCol{
				CategoryCol: tmpfc.GetCatc(),
			}
			return &irpb.FeatureColumn{
				FeatureColumn: &irpb.FeatureColumn_Embc{
					Embc: &irpb.EmbeddingColumn{
						CategoryColumn: embcatc,
						Dimension:      int32(emb.Dimension),
						Combiner:       emb.Combiner,
						Initializer:    emb.Initializer,
					},
				},
			}, nil
		}
		embseqcatc := &irpb.EmbeddingColumn_SeqCategoryCol{
			SeqCategoryCol: tmpfc.GetSeqcatc(),
		}
		return &irpb.FeatureColumn{
			FeatureColumn: &irpb.FeatureColumn_Embc{
				Embc: &irpb.EmbeddingColumn{
					CategoryColumn: embseqcatc,
					Dimension:      int32(emb.Dimension),
					Combiner:       emb.Combiner,
					Initializer:    emb.Initializer,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported feature column type %v", fc)
	}
}

// TrainIRToProto convert parsed TrainIR to a protobuf format
// TODO(typhoonzero): add PredictIR, AnalyzeIR
func TrainIRToProto(ir *TrainIR, sess *pb.Session) (*irpb.TrainIR, error) {
	attrs := make(map[string]string)
	for k, v := range ir.Attributes {
		attrs[k] = attrToPythonValue(v)
	}
	features := make(map[string]*irpb.FeatureColumnList)
	for target, fclist := range ir.Features {
		pbfclist := &irpb.FeatureColumnList{
			FeatureColumns: []*irpb.FeatureColumn{},
		}
		for _, fc := range fclist {
			pbfc, err := featureColumnToPb(fc)
			if err != nil {
				return nil, err
			}
			pbfclist.FeatureColumns = append(
				pbfclist.FeatureColumns,
				pbfc,
			)
		}
		features[target] = pbfclist
	}

	labelFM := ir.Label.GetFieldMeta()[0]
	label := &irpb.FeatureColumn{
		FeatureColumn: &irpb.FeatureColumn_Nc{
			Nc: &irpb.NumericColumn{
				FieldMeta: fieldMetaToPbMeta(labelFM),
			},
		},
	}

	sessIR := &irpb.Session{
		Token:            sess.GetToken(),
		DbConnStr:        sess.GetDbConnStr(),
		ExitOnSubmit:     sess.GetExitOnSubmit(),
		UserId:           sess.GetUserId(),
		HiveLocation:     sess.GetHiveLocation(),
		HdfsNamenodeAddr: sess.GetHdfsNamenodeAddr(),
		HdfsUser:         sess.GetHdfsUser(),
		HdfsPass:         sess.GetHdfsPass(),
	}

	ret := &irpb.TrainIR{
		Datasource:       ir.DataSource,
		Select:           ir.Select,
		ValidationSelect: ir.ValidationSelect,
		Estimator:        ir.Estimator,
		Attributes:       attrs,
		Features:         features,
		Label:            label,
		Session:          sessIR,
	}
	return ret, nil
}
