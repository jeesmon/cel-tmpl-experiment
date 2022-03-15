package main

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter/functions"
	"github.com/google/cel-policy-templates-go/policy"
	"github.com/google/cel-policy-templates-go/policy/runtime"
	eventspb "github.com/jeesmon/cel-tmpl-experiment/events"
	"github.com/jeesmon/cel-tmpl-experiment/utils"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func main() {
	Funcs := []*functions.Overload{
		{
			Operator: "has_uid_boolean",
			Binary: func(lhs, rhs ref.Val) ref.Val {
				v, err := lhs.ConvertToNative(reflect.TypeOf(&eventspb.StudyRevisionEvent{}))
				if err != nil {
					return types.Bool(false)
				}
				event := v.(*eventspb.StudyRevisionEvent)
				uid := rhs.(types.String).Value().(string)

				if event.Study.StudyInstanceUID == uid {
					return types.Bool(true)
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
			decls.NewVar("tag", decls.NewMapType(decls.String, decls.String)),
			decls.NewVar("event", decls.NewObjectType("events.StudyRevisionEvent")),
			decls.NewVar("uid", decls.String),
			decls.NewFunction("hasUID",
				decls.NewOverload("has_uid_boolean",
					[]*exprpb.Type{decls.NewObjectType("events.StudyRevisionEvent"), decls.String},
					decls.Bool,
				),
			),
		),
	)

	opts := []policy.EngineOption{
		policy.StandardExprEnv(env),
		policy.RuntimeTemplateOptions(
			runtime.Functions(Funcs...),
			runtime.NewOrAggregator("filter.allow"),
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
		"tag": map[string]string{
			"group":   "g2",
			"element": "e2",
		},
		"event": &eventspb.StudyRevisionEvent{
			Study: &eventspb.DicomStudy{
				StudyInstanceUID: "123",
			},
		},
		"uid": "123",
	}

	decisions, err := engine.EvalAll(input)
	if err != nil {
		panic(err)
	}

	for _, dec := range decisions {
		fmt.Printf("%v\n", dec)
	}
}
