package main

import (
	"bytes"
	"fmt"
	"go/build"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func isSynthetic(edge *callgraph.Edge) bool {
	return edge.Caller.Func.Pkg == nil || edge.Callee.Func.Synthetic != ""
}

func inStd(node *callgraph.Node) bool {
	pkg, _ := build.Import(node.Func.Pkg.Pkg.Path(), "", 0)
	return pkg.Goroot
}

///MYCODE
//	get default attribute
func defaultAttr(label string) dotAttrs {
	attrs := make(dotAttrs)
	attrs["fillcolor"] = "lightblue"
	attrs["label"] = label
	attrs["style"] = "dotted,filled"
	attrs["tooltip"] = ""
	return attrs
}

///MYCODE
//	get default node given id
func defaultNode(id string) *dotNode {
	return &dotNode{
		ID:    id,
		Attrs: defaultAttr(id),
	}
}

///MYCODE
//	get default edge given caller and callee
func defaultEdge(caller *dotNode, callee *dotNode) *dotEdge {
	return &dotEdge{
		From:  caller,
		To:    callee,
		Attrs: defaultAttr(""),
	}
}

func printOutput(
	prog *ssa.Program,
	mainPkg *types.Package,
	cg *callgraph.Graph,
	focusPkg *types.Package,
	limitPaths,
	ignorePaths,
	includePaths []string,
	groupBy []string,
	nostd,
	nointer bool,
) ([]byte, error) {
	var groupType, groupPkg bool
	for _, g := range groupBy {
		switch g {
		case "pkg":
			groupPkg = true
		case "type":
			groupType = true
		}
	}

	cluster := NewDotCluster("focus")
	cluster.Attrs = dotAttrs{
		"bgcolor":   "white",
		"label":     "",
		"labelloc":  "t",
		"labeljust": "c",
		"fontsize":  "18",
	}
	if focusPkg != nil {
		cluster.Attrs["bgcolor"] = "#e6ecfa"
		cluster.Attrs["label"] = focusPkg.Name()
	}

	var (
		nodes []*dotNode
		edges []*dotEdge
	)

	nodeMap := make(map[string]*dotNode)
	edgeMap := make(map[string]*dotEdge)

	cg.DeleteSyntheticNodes()

	logf("%d limit prefixes: %v", len(limitPaths), limitPaths)
	logf("%d ignore prefixes: %v", len(ignorePaths), ignorePaths)
	logf("%d include prefixes: %v", len(includePaths), includePaths)
	logf("no std packages: %v", nostd)

	var isFocused = func(edge *callgraph.Edge) bool {
		caller := edge.Caller
		callee := edge.Callee
		if focusPkg != nil && (caller.Func.Pkg.Pkg.Path() == focusPkg.Path() || callee.Func.Pkg.Pkg.Path() == focusPkg.Path()) {
			return true
		}
		fromFocused := false
		toFocused := false
		for _, e := range caller.In {
			if !isSynthetic(e) && focusPkg != nil &&
				e.Caller.Func.Pkg.Pkg.Path() == focusPkg.Path() {
				fromFocused = true
				break
			}
		}
		for _, e := range callee.Out {
			if !isSynthetic(e) && focusPkg != nil &&
				e.Callee.Func.Pkg.Pkg.Path() == focusPkg.Path() {
				toFocused = true
				break
			}
		}
		if fromFocused && toFocused {
			logf("edge semi-focus: %s", edge)
			return true
		}
		return false
	}

	var inIncludes = func(node *callgraph.Node) bool {
		pkgPath := node.Func.Pkg.Pkg.Path()
		for _, p := range includePaths {
			if strings.HasPrefix(pkgPath, p) {
				return true
			}
		}
		return false
	}

	var inLimits = func(node *callgraph.Node) bool {
		pkgPath := node.Func.Pkg.Pkg.Path()
		for _, p := range limitPaths {
			if strings.HasPrefix(pkgPath, p) {
				return true
			}
		}
		return false
	}

	var inIgnores = func(node *callgraph.Node) bool {
		pkgPath := node.Func.Pkg.Pkg.Path()
		for _, p := range ignorePaths {
			if strings.HasPrefix(pkgPath, p) {
				return true
			}
		}
		return false
	}

	var isInter = func(edge *callgraph.Edge) bool {
		//caller := edge.Caller
		callee := edge.Callee
		if callee.Func.Object() != nil && !callee.Func.Object().Exported() {
			return true
		}
		return false
	}

	count := 0
	err := callgraph.GraphVisitEdges(cg, func(edge *callgraph.Edge) error {
		count++

		caller := edge.Caller
		callee := edge.Callee

		posCaller := prog.Fset.Position(caller.Func.Pos())
		posCallee := prog.Fset.Position(callee.Func.Pos())
		posEdge := prog.Fset.Position(edge.Pos())
		//fileCaller := fmt.Sprintf("%s:%d", posCaller.Filename, posCaller.Line)
		filenameCaller := filepath.Base(posCaller.Filename)

		// omit synthetic calls
		if isSynthetic(edge) {
			return nil
		}

		callerPkg := caller.Func.Pkg.Pkg
		calleePkg := callee.Func.Pkg.Pkg

		// focus specific pkg
		if focusPkg != nil &&
			!isFocused(edge) {
			return nil
		}

		// omit std
		if nostd &&
			(inStd(caller) || inStd(callee)) {
			return nil
		}

		// omit inter
		if nointer && isInter(edge) {
			return nil
		}

		include := false
		// include path prefixes
		if len(includePaths) > 0 &&
			(inIncludes(caller) || inIncludes(callee)) {
			logf("include: %s -> %s", caller, callee)
			include = true
		}

		if !include {
			// limit path prefixes
			if len(limitPaths) > 0 &&
				(!inLimits(caller) || !inLimits(callee)) {
				logf("NOT in limit: %s -> %s", caller, callee)
				return nil
			}

			// ignore path prefixes
			if len(ignorePaths) > 0 &&
				(inIgnores(caller) || inIgnores(callee)) {
				logf("IS ignored: %s -> %s", caller, callee)
				return nil
			}
		}

		//var buf bytes.Buffer
		//data, _ := json.MarshalIndent(caller.Func, "", " ")
		//logf("call node: %s -> %s\n %v", caller, callee, string(data))
		logf("call node: %s -> %s (%s -> %s) %v\n", caller.Func.Pkg, callee.Func.Pkg, caller, callee, filenameCaller)

		var sprintNode = func(node *callgraph.Node, isCaller bool) *dotNode {
			// only once
			key := node.Func.String()
			nodeTooltip := ""

			fileCaller := fmt.Sprintf("%s:%d", filepath.Base(posCaller.Filename), posCaller.Line)
			fileCallee := fmt.Sprintf("%s:%d", filepath.Base(posCallee.Filename), posCallee.Line)

			if isCaller {
				nodeTooltip = fmt.Sprintf("%s | defined in %s", node.Func.String(), fileCaller)
			} else {
				nodeTooltip = fmt.Sprintf("%s | defined in %s", node.Func.String(), fileCallee)
			}

			if n, ok := nodeMap[key]; ok {
				return n
			}

			// is focused
			isFocused := focusPkg != nil &&
				node.Func.Pkg.Pkg.Path() == focusPkg.Path()
			attrs := make(dotAttrs)

			// node label
			label := node.Func.RelString(node.Func.Pkg.Pkg)

			// func signature
			sign := node.Func.Signature
			if node.Func.Parent() != nil {
				sign = node.Func.Parent().Signature
			}

			// omit type from label
			if groupType && sign.Recv() != nil {
				parts := strings.Split(label, ".")
				label = parts[len(parts)-1]
			}

			pkg, _ := build.Import(node.Func.Pkg.Pkg.Path(), "", 0)
			// set node color
			if isFocused {
				attrs["fillcolor"] = "lightblue"
			} else if pkg.Goroot {
				attrs["fillcolor"] = "#adedad"
			} else {
				attrs["fillcolor"] = "moccasin"
			}

			// include pkg name
			if !groupPkg && !isFocused {
				label = fmt.Sprintf("%s\n%s", node.Func.Pkg.Pkg.Name(), label)
			}

			attrs["label"] = label

			// func styles
			if node.Func.Parent() != nil {
				attrs["style"] = "dotted,filled"
			} else if node.Func.Object() != nil && node.Func.Object().Exported() {
				attrs["penwidth"] = "1.5"
			} else {
				attrs["penwidth"] = "0.5"
			}

			c := cluster

			// group by pkg
			if groupPkg && !isFocused {
				label := node.Func.Pkg.Pkg.Name()
				if pkg.Goroot {
					label = node.Func.Pkg.Pkg.Path()
				}
				key := node.Func.Pkg.Pkg.Path()
				if _, ok := c.Clusters[key]; !ok {
					c.Clusters[key] = &dotCluster{
						ID:       key,
						Clusters: make(map[string]*dotCluster),
						Attrs: dotAttrs{
							"penwidth":  "0.8",
							"fontsize":  "16",
							"label":     label,
							"style":     "filled",
							"fillcolor": "lightyellow",
							"URL":       fmt.Sprintf("/?f=%s", key),
							"fontname":  "Tahoma bold",
							"tooltip":   fmt.Sprintf("package: %s", key),
							"rank":      "sink",
						},
					}
					if pkg.Goroot {
						c.Clusters[key].Attrs["fillcolor"] = "#E0FFE1"
					}
				}
				c = c.Clusters[key]
			}

			// group by type
			if groupType && sign.Recv() != nil {
				label := strings.Split(node.Func.RelString(node.Func.Pkg.Pkg), ".")[0]
				key := sign.Recv().Type().String()
				if _, ok := c.Clusters[key]; !ok {
					c.Clusters[key] = &dotCluster{
						ID:       key,
						Clusters: make(map[string]*dotCluster),
						Attrs: dotAttrs{
							"penwidth":  "0.5",
							"fontsize":  "15",
							"fontcolor": "#222222",
							"label":     label,
							"labelloc":  "b",
							"style":     "rounded,filled",
							"fillcolor": "wheat2",
							"tooltip":   fmt.Sprintf("type: %s", key),
						},
					}
					if isFocused {
						c.Clusters[key].Attrs["fillcolor"] = "lightsteelblue"
					} else if pkg.Goroot {
						c.Clusters[key].Attrs["fillcolor"] = "#c2e3c2"
					}
				}
				c = c.Clusters[key]
			}

			attrs["tooltip"] = nodeTooltip

			n := &dotNode{
				ID:    node.Func.String(),
				Attrs: attrs,
			}

			if c != nil {
				c.Nodes = append(c.Nodes, n)
			} else {
				nodes = append(nodes, n)
			}

			nodeMap[key] = n
			return n
		}
		callerNode := sprintNode(edge.Caller, true)
		calleeNode := sprintNode(edge.Callee, false)

		// edges
		attrs := make(dotAttrs)

		// dynamic call
		if edge.Site != nil && edge.Site.Common().StaticCallee() == nil {
			attrs["style"] = "dashed"
		}

		// go & defer calls
		switch edge.Site.(type) {
		case *ssa.Go:
			attrs["arrowhead"] = "normalnoneodot"
		case *ssa.Defer:
			attrs["arrowhead"] = "normalnoneodiamond"
		}

		// colorize calls outside focused pkg
		if focusPkg != nil &&
			(calleePkg.Path() != focusPkg.Path() || callerPkg.Path() != focusPkg.Path()) {
			attrs["color"] = "saddlebrown"
		}

		// use position in file where callee is called as tooltip for the edge
		fileEdge := fmt.Sprintf(
			"at %s:%d: calling [%s]",
			filepath.Base(posEdge.Filename),
			posEdge.Line,
			edge.Callee.Func.String(),
		)

		// omit duplicate calls, except for tooltip enhancements
		key := fmt.Sprintf("%s = %s => %s", caller.Func, edge.Description(), callee.Func)
		if _, ok := edgeMap[key]; !ok {
			attrs["tooltip"] = fileEdge
			e := &dotEdge{
				From:  callerNode,
				To:    calleeNode,
				Attrs: attrs,
			}
			edgeMap[key] = e
		} else {
			// make sure, tooltip is created correctly
			if _, okk := edgeMap[key].Attrs["tooltip"]; !okk {
				edgeMap[key].Attrs["tooltip"] = fileEdge
			} else {
				edgeMap[key].Attrs["tooltip"] = fmt.Sprintf(
					"%s\n%s",
					edgeMap[key].Attrs["tooltip"],
					fileEdge,
				)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// get edges form edgeMap
	for _, e := range edgeMap {
		e.From.Attrs["tooltip"] = fmt.Sprintf(
			"%s\n%s",
			e.From.Attrs["tooltip"],
			e.Attrs["tooltip"],
		)
		edges = append(edges, e)
	}

	logf("%d/%d edges", len(edges), count)

	dotg := &dotGraph{
		Title:   mainPkg.Path(),
		Minlen:  minlen,
		Cluster: cluster,
		Nodes:   nodes,
		Edges:   edges,
		Options: map[string]string{
			"minlen":    fmt.Sprint(minlen),
			"nodesep":   fmt.Sprint(nodesep),
			"nodeshape": fmt.Sprint(nodeshape),
			"nodestyle": fmt.Sprint(nodestyle),
			"rankdir":   fmt.Sprint(rankdir),
		},
	}

	var go2c map[string]string
	if _, err := os.Stat(*c_dot_callgraph); err == nil {
		go2c = getGO2Cmap(prog)
		dotg = addCGOdotGraph(go2c, dotg)
	} else {
		fmt.Printf("%s does not exist\n", *c_dot_callgraph)
	}

	var buf bytes.Buffer
	if err := dotg.WriteDot(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

///MYCODE
//	need to use `clang -c -emit-llvm XXX.c -o XXX.bc` to generate bitcode files for all C files.
//	then use `llvm-link -S X1.bc X2.bc ... -o tmp.ll` to get integrated llvm IR file
//	last, this function is called to genereate callgraph tmp.ll.callgraph.dot
func getCGOdotGraphBytes() []byte {
	outbyte, _ := ioutil.ReadFile(*c_dot_callgraph)
	return outbyte
}

///MYCODE
//	first find node in C nodes, then find node in Go nodes
//	return node whose name matches fn, nil instead.
func findNode(fn string, dotg *dotGraph) *dotNode {
	fmt.Printf("findNode: %s\n", fn)
	nodes := dotg.Nodes
	fmt.Println("dotg.Nodes")
	for _, node := range nodes {
		fmt.Println(node.ID)
		if node.ID == fn {
			return node
		}
	}
	fmt.Println("dotg.Cluster.Nodes")
	for _, node := range dotg.Cluster.Nodes {
		elems := strings.Split(node.ID, ".")
		real_fn := elems[len(elems)-1]
		fmt.Println(real_fn)
		if real_fn == fn {
			return node
		}
	}
	return nil
}

///MYCODE
//	return map[_Cfunc_XXX] = XXX
func getGO2Cmap(prog *ssa.Program) map[string]string {
	go2c := make(map[string]string)
	for fn := range ssautil.AllFunctions(prog) {
		if strings.HasPrefix(fn.Name(), "_Cfunc_") {
			go2c[fn.Name()] = fn.Name()[7:]
		}
	}
	fmt.Println(go2c)
	return go2c
}

///MYCODE
func trim2Brace(c_fn_str string) string {
	t := len(c_fn_str)
	return c_fn_str[1 : t-1]
}

///MYCODE
//	get name for C's node
func getCFuncName(c_node *cgraph.Node) string {
	return trim2Brace(c_node.Get("label"))
}

///MYCODE
func addCGOdotGraph(go2c map[string]string, dotg *dotGraph) *dotGraph {
	c_graph, err := graphviz.ParseBytes(getCGOdotGraphBytes())
	if err != nil {
		log.Fatal("graphviz.ParseBytes error")
	}
	logf("get CGO callgraph bytes")
	c_edges_num := c_graph.NumberEdges()
	c_node := c_graph.FirstNode()
	c_nodes_num := c_graph.NumberNodes()
	fmt.Println("nodenum", c_nodes_num)
	fmt.Println("edgenum", c_edges_num)
	nodes_map := make(map[string]*dotNode)
	fmt.Println("----------------\nget C's callgraph nodes\n----------------")
	for c_node != nil {
		c_fn_str := getCFuncName(c_node)

		var node *dotNode
		if node = findNode(c_fn_str, dotg); node == nil {
			node = defaultNode(c_fn_str)
			dotg.Nodes = append(dotg.Nodes, node)
		}
		nodes_map[c_fn_str] = node
		fmt.Println(c_fn_str)
		c_node = c_graph.NextNode(c_node)
	}
	fmt.Println("--------------------\nadd out edges\n--------------------")
	c_node = c_graph.FirstNode()
	for c_node != nil {
		caller := nodes_map[getCFuncName(c_node)]
		out_edge := c_graph.FirstOut(c_node)
		for out_edge != nil {
			callee := nodes_map[getCFuncName(out_edge.Node())]
			dotg.Edges = append(dotg.Edges, defaultEdge(caller, callee))
			out_edge = c_graph.NextOut(out_edge)
		}
		c_node = c_graph.NextNode(c_node)
	}
	fmt.Println("-----------------\nadd Go2C edges\n-----------------")
	for _Cfunc_XXX, XXX := range go2c {
		caller := findNode(_Cfunc_XXX, dotg)
		callee := nodes_map[XXX]
		edge := defaultEdge(caller, callee)
		dotg.Edges = append(dotg.Edges, edge)
	}
	return dotg
}
