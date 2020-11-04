package parser

import (
	"fmt"
	"testing"
)

func TestScript(t *testing.T) {
	{
		xs := Parser([]byte("new \"abc\" 1000000000 8"))
		for _, s := range xs {
			fmt.Printf("%s\n", s.name)
			for i, v := range s.args {
				fmt.Printf("%d argument: %v\n", i, v.value)
			}
		}
	}
	{
		xs := Parser([]byte("mint \"abc\" 20"))
		for _, s := range xs {
			fmt.Printf("%s\n", s.name)
			for i, v := range s.args {
				fmt.Printf("%d argument: %v\n", i, v.value)
			}
		}
	}
	{
		xs := Parser([]byte("transfer \"abc\" 10 \"fff\""))
		for _, s := range xs {
			fmt.Printf("%s\n", s.name)
			for i, v := range s.args {
				fmt.Printf("%d argument: %v\n", i, v.value)
			}
		}
	}
	{
		xs := Parser([]byte("freeze \"abc\" \"fff\""))
		for _, s := range xs {
			fmt.Printf("%s\n", s.name)
			for i, v := range s.args {
				fmt.Printf("%d argument: %v\n", i, v.value)
			}
		}
	}
	{
		xs := Parser([]byte("unfreeze \"abc\" \"tom\""))
		for _, s := range xs {
			fmt.Printf("%s\n", s.name)
			for i, v := range s.args {
				fmt.Printf("%d argument: %v\n", i, v.value)
			}
		}
	}
	{
		xs := Parser([]byte("rate \"abc\" 123"))
		for _, s := range xs {
			fmt.Printf("%s\n", s.name)
			for i, v := range s.args {
				fmt.Printf("%d argument: %v\n", i, v.value)
			}
		}
	}
	{
		xs := Parser([]byte("post \"abc\" \"{\"price\": 100, \"hash\": \"robin\""))
		for _, s := range xs {
			fmt.Printf("%s\n", s.name)
			for i, v := range s.args {
				fmt.Printf("%d argument: %v\n", i, v.value)
			}
		}
	}
}
