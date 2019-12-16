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

package ir

import (
	"fmt"

	pb "sqlflow.org/sqlflow/pkg/proto"
)

func attrToPB(attr interface{}) (*pb.Attribute, error) {
	switch attr.(type) {
	case int:
		return &pb.Attribute{
			Attribute: &pb.Attribute_I{I: int32(attr.(int))},
		}, nil
	case float32:
		return &pb.Attribute{
			Attribute: &pb.Attribute_F{F: attr.(float32)},
		}, nil
	case []int:
		il := &pb.Attribute_IntList{Il: toInt32List(attr.([]int))}
		return &pb.Attribute{
			Attribute: &pb.Attribute_Il{Il: il},
		}, nil
		// TODO(typhoonzero): support []float etc.
	case []interface{}:
		tmplist := attr.([]interface{})
		if len(tmplist) > 0 {
			if _, ok := tmplist[0].(int); ok {
				intlist := []int{}
				for _, v := range tmplist {
					intlist = append(intlist, v.(int))
				}
				il := &pb.Attribute_IntList{Il: toInt32List(intlist)}
				return &pb.Attribute{
					Attribute: &pb.Attribute_Il{Il: il},
				}, nil
			}
		}
		// TODO(typhoonzero): support []float etc.
		return nil, fmt.Errorf("attribute is []interface{} with len==0")
	case string:
		return &pb.Attribute{
			Attribute: &pb.Attribute_S{S: attr.(string)},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported attribute type: %T", attr)
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

func fieldDescToPBMeta(fm *FieldDesc) *pb.FieldDesc {
	return &pb.FieldDesc{
		Name:       fm.Name,
		Dtype:      dtypeToString(fm.DType),
		Delimiter:  fm.Delimiter,
		Shape:      toInt32List(fm.Shape),
		IsSparse:   fm.IsSparse,
		Vocabulary: fm.Vocabulary,
		MaxID:      int32(fm.MaxID),
	}
}

func featureColumnToPb(fc FeatureColumn) (*pb.FeatureColumn, error) {
	switch fc.(type) {
	case *NumericColumn:
		nc := &pb.FeatureColumn{
			FeatureColumn: &pb.FeatureColumn_Nc{
				Nc: &pb.NumericColumn{
					FieldDesc: fieldDescToPBMeta(fc.GetFieldDesc()[0]),
				},
			},
		}
		return nc, nil
	case *BucketColumn:
		fm := fc.GetFieldDesc()[0]
		bc := &pb.FeatureColumn{
			FeatureColumn: &pb.FeatureColumn_Bc{
				Bc: &pb.BucketColumn{
					SourceColumn: &pb.NumericColumn{
						FieldDesc: fieldDescToPBMeta(fm),
					},
					Boundaries: toInt32List(fc.(*BucketColumn).Boundaries),
				},
			},
		}
		return bc, nil
	case *CrossColumn:
		cc := fc.(*CrossColumn)
		pbkeys := []*pb.FeatureColumn{}
		for _, key := range cc.Keys {
			tmpfc, err := featureColumnToPb(key.(FeatureColumn))
			if err != nil {
				return nil, err
			}
			pbkeys = append(pbkeys, tmpfc)
		}
		pbcc := &pb.FeatureColumn{
			FeatureColumn: &pb.FeatureColumn_Cc{
				Cc: &pb.CrossColumn{
					Keys:           pbkeys,
					HashBucketSize: int32(cc.HashBucketSize),
				},
			},
		}
		return pbcc, nil
	case *CategoryIDColumn:
		catc := fc.(*CategoryIDColumn)
		pbcatc := &pb.FeatureColumn{
			FeatureColumn: &pb.FeatureColumn_Catc{
				Catc: &pb.CategoryIDColumn{
					FieldDesc:  fieldDescToPBMeta(fc.GetFieldDesc()[0]),
					BucketSize: int32(catc.BucketSize),
				},
			},
		}
		return pbcatc, nil
	case *SeqCategoryIDColumn:
		seqcatc := fc.(*SeqCategoryIDColumn)
		pbseqcatc := &pb.FeatureColumn{
			FeatureColumn: &pb.FeatureColumn_Seqcatc{
				Seqcatc: &pb.SeqCategoryIDColumn{
					FieldDesc:  fieldDescToPBMeta(fc.GetFieldDesc()[0]),
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
			embcatc := &pb.EmbeddingColumn_CategoryCol{
				CategoryCol: tmpfc.GetCatc(),
			}
			return &pb.FeatureColumn{
				FeatureColumn: &pb.FeatureColumn_Embc{
					Embc: &pb.EmbeddingColumn{
						CategoryColumn: embcatc,
						Dimension:      int32(emb.Dimension),
						Combiner:       emb.Combiner,
						Initializer:    emb.Initializer,
					},
				},
			}, nil
		}
		embseqcatc := &pb.EmbeddingColumn_SeqCategoryCol{
			SeqCategoryCol: tmpfc.GetSeqcatc(),
		}
		return &pb.FeatureColumn{
			FeatureColumn: &pb.FeatureColumn_Embc{
				Embc: &pb.EmbeddingColumn{
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

// AttributesToProto convert attributes from IR to protobuf format
func AttributesToProto(attrsIR map[string]interface{}) (map[string]*pb.Attribute, error) {
	attrs := make(map[string]*pb.Attribute)
	for k, v := range attrsIR {
		a, err := attrToPB(v)
		if err != nil {
			return nil, err
		}
		attrs[k] = a
	}
	return attrs, nil
}

// TrainStmtToProto convert parsed TrainStmt to a protobuf format
func TrainStmtToProto(trainStmt *TrainStmt, sess *pb.Session) (*pb.TrainStmt, error) {
	attrs, err := AttributesToProto(trainStmt.Attributes)
	if err != nil {
		return nil, err
	}
	features := make(map[string]*pb.FeatureColumnList)
	for target, fclist := range trainStmt.Features {
		pbfclist := &pb.FeatureColumnList{
			FeatureColumns: []*pb.FeatureColumn{},
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

	labelFM := trainStmt.Label.GetFieldDesc()[0]
	label := &pb.FeatureColumn{
		FeatureColumn: &pb.FeatureColumn_Nc{
			Nc: &pb.NumericColumn{
				FieldDesc: fieldDescToPBMeta(labelFM),
			},
		},
	}

	ret := &pb.TrainStmt{
		Datasource:       trainStmt.DataSource,
		Select:           trainStmt.Select,
		ValidationSelect: trainStmt.ValidationSelect,
		Estimator:        trainStmt.Estimator,
		Attributes:       attrs,
		Features:         features,
		Label:            label,
		Session:          sess,
		Into:             trainStmt.Into,
	}
	return ret, nil
}

// PredictStmtToProto convert parsed predictStmt to a protobuf format
func PredictStmtToProto(predictStmt *PredictStmt, sess *pb.Session) (*pb.PredictStmt, error) {
	trainStmt, err := TrainStmtToProto(predictStmt.TrainStmt, sess)
	if err != nil {
		return nil, err
	}
	attrs, err := AttributesToProto(predictStmt.Attributes)
	if err != nil {
		return nil, err
	}
	return &pb.PredictStmt{
		Datasource:   predictStmt.DataSource,
		Select:       predictStmt.Select,
		ResultTable:  predictStmt.ResultTable,
		ResultColumn: predictStmt.ResultColumn,
		Attributes:   attrs,
		TrainIr:      trainStmt,
	}, nil
}

// AnalyzeStmtToProto convert parsed AnalyzeStmt to a protobuf format
func AnalyzeStmtToProto(analyzeStmt *AnalyzeStmt, sess *pb.Session) (*pb.AnalyzeStmt, error) {
	trainStmt, err := TrainStmtToProto(analyzeStmt.TrainStmt, sess)
	if err != nil {
		return nil, err
	}
	attrs, err := AttributesToProto(analyzeStmt.Attributes)
	if err != nil {
		return nil, err
	}
	return &pb.AnalyzeStmt{
		Datasource: analyzeStmt.DataSource,
		Select:     analyzeStmt.Select,
		Attributes: attrs,
		Explainer:  analyzeStmt.Explainer,
		TrainIr:    trainStmt,
	}, nil
}
