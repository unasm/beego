// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var errSkipField = errors.New("skip field")

// field info collection
type fields struct {
	// 主键的 字段信息
	pk *fieldInfo

	// 数据库字段 到具体字段的 映射
	columns map[string]*fieldInfo

	// Id, Ip 数据库 字段的首字母大写
	fields map[string]*fieldInfo

	//ip，id 等，key 应该是 数据库字段的名称
	fieldsLow     map[string]*fieldInfo
	fieldsByType  map[int][]*fieldInfo
	fieldsRel     []*fieldInfo
	fieldsReverse []*fieldInfo
	fieldsDB      []*fieldInfo
	rels          []*fieldInfo
	// 具体的数据库 每一列的字段名称， way,dim,reason 等
	orders []string
	// 数据库列的名称，[id boolean char text j_s_o_n jsonb time date datetime byte rune int int8 int16 int32 int64 uint uint8 uint16 uint32 uint64 float32 float64 decimal]
	dbcols []string
}

// add field info
func (f *fields) Add(fi *fieldInfo) (added bool) {
	if f.fields[fi.name] == nil && f.columns[fi.column] == nil {
		f.columns[fi.column] = fi
		f.fields[fi.name] = fi
		f.fieldsLow[strings.ToLower(fi.name)] = fi
	} else {
		return
	}
	if _, ok := f.fieldsByType[fi.fieldType]; ok == false {
		f.fieldsByType[fi.fieldType] = make([]*fieldInfo, 0)
	}
	f.fieldsByType[fi.fieldType] = append(f.fieldsByType[fi.fieldType], fi)
	f.orders = append(f.orders, fi.column)
	if fi.dbcol {
		f.dbcols = append(f.dbcols, fi.column)
		f.fieldsDB = append(f.fieldsDB, fi)
	}
	if fi.rel {
		f.fieldsRel = append(f.fieldsRel, fi)
	}
	if fi.reverse {
		f.fieldsReverse = append(f.fieldsReverse, fi)
	}
	return true
}

// get field info by name
func (f *fields) GetByName(name string) *fieldInfo {
	return f.fields[name]
}

// get field info by column name
func (f *fields) GetByColumn(column string) *fieldInfo {
	return f.columns[column]
}

// get field info by string, name is prior
func (f *fields) GetByAny(name string) (*fieldInfo, bool) {
	if fi, ok := f.fields[name]; ok {
		return fi, ok
	}
	if fi, ok := f.fieldsLow[strings.ToLower(name)]; ok {
		return fi, ok
	}
	if fi, ok := f.columns[name]; ok {
		return fi, ok
	}
	return nil, false
}

// create new field info collection
func newFields() *fields {
	f := new(fields)
	f.fields = make(map[string]*fieldInfo)
	f.fieldsLow = make(map[string]*fieldInfo)
	f.columns = make(map[string]*fieldInfo)
	f.fieldsByType = make(map[int][]*fieldInfo)
	return f
}

// single field info
type fieldInfo struct {
	//field 反向的归属model位置
	mi *modelInfo
	// 一般情况下为 单个值的 数组，表示该field在结构体中的位置
	fieldIndex []int

	//字段类型, models_fields.go 中约定的Type类型，在插入或者读出的时候，用于转换为对应的类型
	fieldType           int
	dbcol               bool
	inModel             bool
	name                string
	fullName            string
	column              string
	addrValue           reflect.Value
	sf                  reflect.StructField
	auto                bool
	pk                  bool
	null                bool
	index               bool
	unique              bool
	colDefault          bool
	initial             StrTo
	size                int
	toText              bool
	autoNow             bool
	autoNowAdd          bool
	rel                 bool
	reverse             bool
	reverseField        string
	reverseFieldInfo    *fieldInfo
	reverseFieldInfoTwo *fieldInfo
	reverseFieldInfoM2M *fieldInfo
	relTable            string
	relThrough          string
	relThroughModelInfo *modelInfo
	relModelInfo        *modelInfo
	digits              int
	decimals            int
	isFielder           bool
	onDelete            string
}

// new field info
func newFieldInfo(mi *modelInfo, field reflect.Value, sf reflect.StructField, mName string) (fi *fieldInfo, err error) {
	var (
		tag       string
		tagValue  string
		initial   StrTo
		fieldType int
		attrs     map[string]bool
		tags      map[string]string
		addrField reflect.Value
	)

	fi = new(fieldInfo)

	addrField = field
	if field.CanAddr() && field.Kind() != reflect.Ptr {
		addrField = field.Addr()
		if _, ok := addrField.Interface().(Fielder); !ok {
			if field.Kind() == reflect.Slice {
				addrField = field
			}
		}
	}

	//sf.Tag的值 orm:"digits(8);decimals(4);null"
	// defaultStructTagName 是orm ，也就是获取orm 相关的字符串中的值,处理orm
	parseStructTag(sf.Tag.Get(defaultStructTagName), &attrs, &tags)

	if _, ok := attrs["-"]; ok {
		return nil, errSkipField
	}

	digits := tags["digits"]
	decimals := tags["decimals"]
	size := tags["size"]
	onDelete := tags["on_delete"]

	initial.Clear()
	if v, ok := tags["default"]; ok {
		initial.Set(v)
	}

checkType:
	switch f := addrField.Interface().(type) {
	case Fielder:
		fi.isFielder = true
		if field.Kind() == reflect.Ptr {
			err = fmt.Errorf("the model Fielder can not be use ptr")
			goto end
		}
		fieldType = f.FieldType()
		if fieldType&IsRelField > 0 {
			err = fmt.Errorf("unsupport rel type custom field")
			goto end
		}
	default:
		tag = "rel"
		tagValue = tags[tag]
		if tagValue != "" {
			switch tagValue {
			case "fk":
				fieldType = RelForeignKey
				break checkType
			case "one":
				fieldType = RelOneToOne
				break checkType
			case "m2m":
				fieldType = RelManyToMany
				if tv := tags["rel_table"]; tv != "" {
					fi.relTable = tv
				} else if tv := tags["rel_through"]; tv != "" {
					fi.relThrough = tv
				}
				break checkType
			default:
				err = fmt.Errorf("error")
				goto wrongTag
			}
		}
		tag = "reverse"
		tagValue = tags[tag]
		if tagValue != "" {
			switch tagValue {
			case "one":
				fieldType = RelReverseOne
				break checkType
			case "many":
				fieldType = RelReverseMany
				if tv := tags["rel_table"]; tv != "" {
					fi.relTable = tv
				} else if tv := tags["rel_through"]; tv != "" {
					fi.relThrough = tv
				}
				break checkType
			default:
				err = fmt.Errorf("error")
				goto wrongTag
			}
		}

		fieldType, err = getFieldType(addrField)
		if err != nil {
			goto end
		}
		if fieldType == TypeCharField {
			switch tags["type"] {
			case "text":
				fieldType = TypeTextField
			case "json":
				fieldType = TypeJSONField
			case "jsonb":
				fieldType = TypeJsonbField
			}
		}
		if fieldType == TypeFloatField && (digits != "" || decimals != "") {
			fieldType = TypeDecimalField
		}
		if fieldType == TypeDateTimeField && tags["type"] == "date" {
			fieldType = TypeDateField
		}
		if fieldType == TypeTimeField && tags["type"] == "time" {
			fieldType = TypeTimeField
		}
	}

	switch fieldType {
	case RelForeignKey, RelOneToOne, RelReverseOne:
		if field.Kind() != reflect.Ptr {
			err = fmt.Errorf("rel/reverse:one field must be *%s", field.Type().Name())
			goto end
		}
	case RelManyToMany, RelReverseMany:
		if field.Kind() != reflect.Slice {
			err = fmt.Errorf("rel/reverse:many field must be slice")
			goto end
		} else {
			if field.Type().Elem().Kind() != reflect.Ptr {
				err = fmt.Errorf("rel/reverse:many slice must be []*%s", field.Type().Elem().Name())
				goto end
			}
		}
	}

	if fieldType&IsFieldType == 0 {
		err = fmt.Errorf("wrong field type")
		goto end
	}

	fi.fieldType = fieldType
	fi.name = sf.Name
	fi.column = getColumnName(fieldType, addrField, sf, tags["column"])
	fi.addrValue = addrField
	fi.sf = sf
	fi.fullName = mi.fullName + mName + "." + sf.Name

	fi.null = attrs["null"]
	fi.index = attrs["index"]
	fi.auto = attrs["auto"]
	fi.pk = attrs["pk"]
	fi.unique = attrs["unique"]

	// Mark object property if there is attribute "default" in the orm configuration
	if _, ok := tags["default"]; ok {
		fi.colDefault = true
	}

	switch fieldType {
	case RelManyToMany, RelReverseMany, RelReverseOne:
		fi.null = false
		fi.index = false
		fi.auto = false
		fi.pk = false
		fi.unique = false
	default:
		fi.dbcol = true
	}

	switch fieldType {
	case RelForeignKey, RelOneToOne, RelManyToMany:
		fi.rel = true
		if fieldType == RelOneToOne {
			fi.unique = true
		}
	case RelReverseMany, RelReverseOne:
		fi.reverse = true
	}

	if fi.rel && fi.dbcol {
		switch onDelete {
		case odCascade, odDoNothing:
		case odSetDefault:
			if initial.Exist() == false {
				err = errors.New("on_delete: set_default need set field a default value")
				goto end
			}
		case odSetNULL:
			if fi.null == false {
				err = errors.New("on_delete: set_null need set field null")
				goto end
			}
		default:
			if onDelete == "" {
				onDelete = odCascade
			} else {
				err = fmt.Errorf("on_delete value expected choice in `cascade,set_null,set_default,do_nothing`, unknown `%s`", onDelete)
				goto end
			}
		}

		fi.onDelete = onDelete
	}

	switch fieldType {
	case TypeBooleanField:
	case TypeCharField, TypeJSONField, TypeJsonbField:
		if size != "" {
			v, e := StrTo(size).Int32()
			if e != nil {
				err = fmt.Errorf("wrong size value `%s`", size)
			} else {
				fi.size = int(v)
			}
		} else {
			fi.size = 255
			fi.toText = true
		}
	case TypeTextField:
		fi.index = false
		fi.unique = false
	case TypeTimeField, TypeDateField, TypeDateTimeField:
		if attrs["auto_now"] {
			fi.autoNow = true
		} else if attrs["auto_now_add"] {
			fi.autoNowAdd = true
		}
	case TypeFloatField:
	case TypeDecimalField:
		d1 := digits
		d2 := decimals
		v1, er1 := StrTo(d1).Int8()
		v2, er2 := StrTo(d2).Int8()
		if er1 != nil || er2 != nil {
			err = fmt.Errorf("wrong digits/decimals value %s/%s", d2, d1)
			goto end
		}
		fi.digits = int(v1)
		fi.decimals = int(v2)
	default:
		switch {
		case fieldType&IsIntegerField > 0:
		case fieldType&IsRelField > 0:
		}
	}

	if fieldType&IsIntegerField == 0 {
		if fi.auto {
			err = fmt.Errorf("non-integer type cannot set auto")
			goto end
		}
	}

	if fi.auto || fi.pk {
		if fi.auto {

			switch addrField.Elem().Kind() {
			case reflect.Int, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint32, reflect.Uint64:
			default:
				err = fmt.Errorf("auto primary key only support int, int32, int64, uint, uint32, uint64 but found `%s`", addrField.Elem().Kind())
				goto end
			}

			fi.pk = true
		}
		fi.null = false
		fi.index = false
		fi.unique = false
	}

	if fi.unique {
		fi.index = false
	}

	if fi.auto || fi.pk || fi.unique || fieldType == TypeTimeField || fieldType == TypeDateField || fieldType == TypeDateTimeField {
		// can not set default
		initial.Clear()
	}

	if initial.Exist() {
		v := initial
		switch fieldType {
		case TypeBooleanField:
			_, err = v.Bool()
		case TypeFloatField, TypeDecimalField:
			_, err = v.Float64()
		case TypeBitField:
			_, err = v.Int8()
		case TypeSmallIntegerField:
			_, err = v.Int16()
		case TypeIntegerField:
			_, err = v.Int32()
		case TypeBigIntegerField:
			_, err = v.Int64()
		case TypePositiveBitField:
			_, err = v.Uint8()
		case TypePositiveSmallIntegerField:
			_, err = v.Uint16()
		case TypePositiveIntegerField:
			_, err = v.Uint32()
		case TypePositiveBigIntegerField:
			_, err = v.Uint64()
		}
		if err != nil {
			tag, tagValue = "default", tags["default"]
			goto wrongTag
		}
	}

	fi.initial = initial
end:
	if err != nil {
		return nil, err
	}
	return
wrongTag:
	return nil, fmt.Errorf("wrong tag format: `%s:\"%s\"`, %s", tag, tagValue, err)
}
