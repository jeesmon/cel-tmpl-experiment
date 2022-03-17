package main

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter/functions"
	"github.com/google/cel-policy-templates-go/policy"
	"github.com/google/cel-policy-templates-go/policy/model"
	"github.com/google/cel-policy-templates-go/policy/runtime"
	eventspb "github.com/jeesmon/cel-tmpl-experiment/events"
	"github.com/jeesmon/cel-tmpl-experiment/utils"
	"github.com/mitchellh/mapstructure"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func main() {
	beg := time.Now()
	Funcs := []*functions.Overload{
		{
			Operator: "has_modality_boolean",
			Function: func(args ...ref.Val) ref.Val {
				v, err := args[0].ConvertToNative(reflect.TypeOf(&eventspb.StudyRevisionEvent{}))
				if err != nil {
					return types.Bool(false)
				}
				event := v.(*eventspb.StudyRevisionEvent)
				scope := args[1].(types.String).Value().(string)
				modality := args[2].(types.String).Value().(string)

				fmt.Printf("scope: %s, modality: %s\n", scope, modality)

				if scope == "series" {
					for _, s := range event.Study.Series {
						if s.Modality == modality {
							return types.Bool(true)
						}
					}
				}

				return types.Bool(false)
			},
		},
	}

	env, _ := cel.NewEnv(
		cel.Types(
			&eventspb.StudyRevisionEvent{},
		),
		cel.Declarations(
			decls.NewVar("event", decls.NewObjectType("events.StudyRevisionEvent")),
			decls.NewVar("scope", decls.String),
			decls.NewVar("modality", decls.String),
			decls.NewFunction("hasModality",
				decls.NewOverload("has_modality_boolean",
					[]*exprpb.Type{decls.NewObjectType("events.StudyRevisionEvent"), decls.String, decls.String},
					decls.Bool,
				),
			),
		),
	)

	opts := []policy.EngineOption{
		policy.StandardExprEnv(env),
		policy.RangeLimit(-1),
		policy.RuntimeTemplateOptions(
			runtime.Functions(Funcs...),
			runtime.NewOrAggregator("protocol.decision"),
			runtime.NewCollectAggregator("algo.decision"),
		),
	}
	engine, err := policy.NewEngine(opts...)
	if err != nil {
		panic(err)
	}

	r := utils.NewReader(".")

	tmplSrc, ok := r.Read("template.yaml")
	if !ok {
		panic("Couldn't read template")
	}

	tmpl, iss := engine.CompileTemplate(tmplSrc)
	if iss.Err() != nil {
		panic(iss.Err())
	}

	err = engine.SetTemplate(tmpl.Metadata.Name, tmpl)
	if err != nil {
		panic(err)
	}

	instSrc, ok := r.Read("instance.yaml")
	if !ok {
		panic("Couldn't read instance")
	}

	inst, iss := engine.CompileInstance(instSrc)
	if iss.Err() != nil {
		panic(iss.Err())
	}

	engine.AddInstance(inst)

	input := map[string]interface{}{
		"event": &eventspb.StudyRevisionEvent{
			Source: "clientStorageSpace",
			Study: &eventspb.DicomStudy{
				StudyInstanceUID: "123",
				Series: []*eventspb.DicomSeries{
					{
						SeriesInstanceUID: "1234",
						Modality:          "MR",
					},
				},
			},
		},
	}

	eval := time.Now()
	decisions, err := engine.EvalAll(input)
	if err != nil {
		panic(err)
	}
	end := time.Now()
	fmt.Printf("Eval time: %v\n", end.Sub(eval))
	fmt.Printf("Comple + Eval time: %v\n", end.Sub(beg))

	for _, dec := range decisions {
		printDecision(dec)
	}
}

type Algo struct {
	Name   string
	Scope  string
	Result []Result
}

type Result struct {
	Source   string
	Series   string
	SopClass string
}

func (a Algo) String() string {
	return fmt.Sprintf("name: %s, scope: %s, result: %s", a.Name, a.Scope, a.Result)
}

func (r Result) String() string {
	return fmt.Sprintf("source: %s, series: %s, sopClass: %s", r.Source, r.Series, r.SopClass)
}

func printDecision(dec model.DecisionValue) {
	switch dv := dec.(type) {
	case *model.BoolDecisionValue:
		out := types.Bool(false)
		ntv, _ := dv.Value().ConvertToNative(reflect.TypeOf(out))
		fmt.Printf("%v\n", ntv)
	case *model.ListDecisionValue:
		vals := dv.Values()
		for _, val := range vals {
			ntv, _ := val.ConvertToNative(reflect.TypeOf(map[string]interface{}{}))
			m := ntv.(map[string]interface{})
			algo := Algo{}
			mapstructure.Decode(m, &algo)

			rslt := m["result"].(*model.ListValue)
			for _, e := range rslt.Entries {
				ntv, _ := e.ConvertToNative(reflect.TypeOf(map[string]string{}))
				m := ntv.(map[string]string)
				result := Result{}
				mapstructure.Decode(m, &result)
				algo.Result = append(algo.Result, result)
			}

			fmt.Println(algo)
		}
	}
}
