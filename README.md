# xray - look through your golang objects

[![Build workflow](https://img.shields.io/github/workflow/status/hanlins/xray/Go)](https://img.shields.io/github/workflow/status/hanlins/xray/Go)
[![GoDoc](https://godoc.org/github.com/hanlins/xray?status.svg)](https://godoc.org/github.com/hanlins/xray)

## Introduction

Trouble shooting a project isn't easy, especially when pointers are being used.
It's often difficult to get an idea on an object in runtime.
**Xray** is designed to parse runtime objects, and help user to analyze the objects. The accompany `dot` package, as an example, will generate a dot representation of the object so user could easilly visualize the object.

## Examples

### Default formatted printer

Typically, user could print their objects in string format:
```golang
package main

import "fmt"

type test struct {
	ptr *string
	arr []int
	m   map[string]int
}

func main() {
	str := "deadbeef"
	object := test{
		ptr: &str,
		arr: []int{1, 2, 3},
		m:   map[string]int{"foo": 1, "bar": 2},
	}
	fmt.Printf("object:\n%#v\n", object)
}
```

Then this is what you got:

```
object:
main.test{ptr:(*string)(0xc000010200), arr:[]int{1, 2, 3}, m:map[string]int{"bar":2, "foo":1}}
```

It's not helpful when you want to reason about the pointers, and becomes messy when the struct becomes complicated.

### Xray parsed object

By using **xray**, you can parse the golang object and generate a `dot` representation of it, here's the snipple:

```golang
package main

import (
	"fmt"
	"github.com/hanlins/xray"
	"github.com/hanlins/xray/dot"
)

type test struct {
	ptr *string
	arr []int
	m   map[string]int
}

func main() {
	str := "deadbeef"
	object := test{
		ptr: &str,
		arr: []int{1, 2, 3},
		m:   map[string]int{"foo": 1, "bar": 2},
	}

	s := xray.NewScanner(nil)
	nodeCh := s.Scan(object)
	g, _ := dot.Draw(dot.NewGraphInfo(s), nodeCh, nil)
	fmt.Printf("dot object:\n%s\n", g)
}
```

You can use the `dot` binary (need to install `graphviz` package) locally or some online generator (e.g. [viz-js](http://viz-js.com/)) to generate an image, and use it in your report. Here's the generated image:
![Alt text](doc/ex1.png?raw=true "Example 1")