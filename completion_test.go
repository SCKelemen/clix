package clix

import (
	"reflect"
	"testing"
)

func TestSetCompletion(t *testing.T) {
	fs := NewFlagSet("t")
	var region string
	fs.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "region"}, Value: &region})
	if fs.SetCompletion("nope", CompleteValues("x")) {
		t.Fatal("SetCompletion on a missing flag reported success")
	}
	if !fs.SetCompletion("region", CompleteValues("eu", "us")) {
		t.Fatal("SetCompletion failed")
	}
	f := fs.lookup("region")
	got := f.Completer()(CompletionRequest{Flag: f})
	want := []Completion{{Value: "eu"}, {Value: "us"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	var nilFlag *Flag
	if nilFlag.Completer() != nil {
		t.Fatal("nil flag has a completer")
	}
}

func TestCompleteOptions(t *testing.T) {
	got := CompleteOptions(
		SelectOption{Label: "Production", Value: "prod", Description: "live traffic"},
		SelectOption{Label: "staging", Value: "staging"},
		SelectOption{Label: "dev"},
	)(CompletionRequest{})
	want := []Completion{
		{Value: "prod", Description: "Production - live traffic"},
		{Value: "staging"},
		{Value: "dev"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
