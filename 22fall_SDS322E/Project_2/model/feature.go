package model

import (
	"log"
	"math"
	"reflect"
	"strconv"
)

type NamingConvention struct {
	PeriodSeparated int `csv:"period_separated"`
	CamelCase       int `csv:"camel_case"`
	SnakeCase       int `csv:"snake_case"`
	PascalCase      int `csv:"pascal_case"`
	AllCaps         int `csv:"all_caps"`
	Total           int `csv:"total"`
}

type NamingConventionProp struct {
	PeriodSeparated float64 `csv:"period_separated"`
	CamelCase       float64 `csv:"camel_case"`
	SnakeCase       float64 `csv:"snake_case"`
	PascalCase      float64 `csv:"pascal_case"`
	AllCaps         float64 `csv:"all_caps"`
}

func (n NamingConvention) Props() NamingConventionProp {
	return NamingConventionProp{
		PeriodSeparated: float64(n.PeriodSeparated) / float64(n.Total),
		CamelCase:       float64(n.CamelCase) / float64(n.Total),
		SnakeCase:       float64(n.SnakeCase) / float64(n.Total),
		PascalCase:      float64(n.PascalCase) / float64(n.Total),
		AllCaps:         float64(n.AllCaps) / float64(n.Total),
	}
}

type F struct {
	Package          string               `csv:"package"`
	Repo             string               `csv:"repo"`
	TitleWords       int                  `csv:"title.words"`
	DescriptionWords int                  `csv:"description.words"`
	PropEqAssign     float64              `csv:"eq_assign.prop"`
	NameVariable     NamingConvention     `csv:"var.naming"`
	NameExport       NamingConvention     `csv:"export.naming"`
	NameRFile        NamingConvention     `csv:"rfile.naming"`
	NameVariableProp NamingConventionProp `csv:"var.naming.prop"`
	NameExportProp   NamingConventionProp `csv:"export.naming.prop"`
	NameRFileProp    NamingConventionProp `csv:"rfile.naming.prop"`
	CallRandomForest int                  `csv:"call.randomForest"`
	CallRpart        int                  `csv:"call.rpart"`
	AvgRTokens       float64              `csv:"avg_r_tokens"`
	RExportNum       int                  `csv:"export.num"`
	RImportToDepend  float64              `csv:"r_import_to_depend"`
	MajorVersion     int                  `csv:"version.major"`
	COverR           float64              `csv:"native.c.prop"`
	FOverR           float64              `csv:"native.f.prop"`
	JOverR           float64              `csv:"native.j.prop"`
	ExtR             float64              `csv:"ext.r"`
	ExtRd            float64              `csv:"ext.rd"`
	ExtRds           float64              `csv:"ext.rds"`
	ExtRda           float64              `csv:"ext.rda"`
}

func (f F) FloatCheck() {
	v := reflect.ValueOf(f)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.CanFloat() {
			val := field.Float()
			if math.IsNaN(val) {
				log.Printf("NaN value in field %s", v.Type().Field(i).Name)
			} else if math.IsInf(val, 0) {
				log.Printf("Inf value in field %s", v.Type().Field(i).Name)
			}
		} else if field.Kind() == reflect.Struct {
			for j := 0; j < field.NumField(); j++ {
				subField := field.Field(j)
				if subField.CanFloat() {
					val := subField.Float()
					if math.IsNaN(val) {
						log.Printf("NaN value in field %s.%s", v.Type().Field(i).Name, field.Type().Field(j).Name)
					} else if math.IsInf(val, 0) {
						log.Printf("Inf value in field %s.%s", v.Type().Field(i).Name, field.Type().Field(j).Name)
					}
				}
			}
		}
	}
}

func (f F) Header() []string {
	t := reflect.TypeOf(f)
	var h []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("csv")
		if field.Type.Kind() == reflect.Struct {
			for j := 0; j < field.Type.NumField(); j++ {
				subField := field.Type.Field(j)
				subTag := subField.Tag.Get("csv")
				h = append(h, tag+".."+subTag)
			}
		} else {
			h = append(h, tag)
		}
	}
	return h
}

type KV struct {
	Key   string
	Value any
}

func (f F) KVPairs() []KV {
	v := reflect.ValueOf(f)
	var kvs []KV
	addField := func(field reflect.Value, key string) {
		switch field.Kind() {
		case reflect.Int:
			kvs = append(kvs, KV{key, int(field.Int())})
		case reflect.Float64:
			kvs = append(kvs, KV{key, field.Float()})
		case reflect.String:
			kvs = append(kvs, KV{key, field.String()})
		}
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldTag := v.Type().Field(i).Tag.Get("csv")
		if field.Kind() == reflect.Struct {
			for j := 0; j < field.NumField(); j++ {
				subField := field.Field(j)
				subFieldTag := field.Type().Field(j).Tag.Get("csv")
				addField(subField, fieldTag+".."+subFieldTag)
			}
		} else {
			tag := v.Type().Field(i).Tag.Get("csv")
			addField(field, tag)
		}
	}
	return kvs
}

func (f F) FieldValues() []string {
	v := reflect.ValueOf(f)
	var vals []string
	addField := func(field reflect.Value) {
		switch field.Kind() {
		case reflect.Int:
			vals = append(vals, strconv.Itoa(int(field.Int())))
		case reflect.Float64:
			vals = append(vals, strconv.FormatFloat(field.Float(), 'f', -1, 64))
		case reflect.String:
			vals = append(vals, field.String())
		}
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() == reflect.Struct {
			for j := 0; j < field.NumField(); j++ {
				subField := field.Field(j)
				addField(subField)
			}
		} else {
			addField(field)
		}
	}
	return vals
}
