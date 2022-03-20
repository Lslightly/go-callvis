//	code to generate cgo _obj files and dot callgraph
package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const buildPathStr = "build"

var fp, _ = filepath.Abs(*c_root_path)
var absBuildPath = fp + "/" + buildPathStr
var absDefaultDotPath = fp + "/callgraph.dot"

///MYCODE
//	generate c's dot format callgraph
func genCdotCallgraph(prog *ssa.Program) error {
	if _, err := os.Stat(*c_root_path); err == nil {
		if err := genObjDir(); err != nil {
			logf("gen obj dir error")
			return err
		}
		if err := genUnifDefAndBitCode(); err != nil {
			logf("gen unifdef and bitcode error")
			return err
		}
		if err := bc2dot(); err != nil {
			logf("gen callgraph error")
			return err
		}
		return nil
	} else {
		fmt.Printf("%s does not exist\n", *c_root_path)
		return err
	}
}

///MYCODE
//	generate _obj dir for each package(dir)
func genObjDir() error {
	gen_obj_args := []string{"tool", "cgo"}
	gen_obj_under_dir_fn := func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() || strings.Contains(path, "_obj") || strings.Contains(path, "build") {
			return nil
		}
		abs_path, _ := filepath.Abs(path)
		rds, _ := ioutil.ReadDir(abs_path)
		for _, fi := range rds {
			if !fi.IsDir() && strings.HasSuffix(fi.Name(), ".go") {
				tmp_args := append(gen_obj_args, abs_path+"/"+fi.Name())
				gen_obj_cmd := exec.Command("go", tmp_args...)
				gen_obj_cmd.Dir = abs_path
				logf(gen_obj_cmd.String())
				if b, err := gen_obj_cmd.CombinedOutput(); err != nil {
					logf(string(b))
					return nil
				} else {
					logf("succeed")
				}
			}
		}
		return nil
	}
	filepath.WalkDir(*c_root_path, gen_obj_under_dir_fn)
	return nil
}

///MYCODE
//	use unifdef to trim macros
func genUnifDefAndBitCode() error {
	if _, err := os.Lstat(absBuildPath); err == nil {
		os.RemoveAll(absBuildPath)
	}
	os.Mkdir(absBuildPath, 0755)
	gen_unifdef_bc_fn := func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() || strings.Contains(path, "build") {
			return nil
		}
		logf("gen unifdef and bitcode visit %s", path)
		abs_path, _ := filepath.Abs(path)
		rds, _ := ioutil.ReadDir(abs_path)
		bit_code_args := []string{"-c", "-emit-llvm"}
		if strings.Contains(path, "_obj") {
			bit_code_args = append(bit_code_args, "-I", abs_path+"/../", "-I", abs_path)
		}
		for _, fi := range rds {
			if fi.IsDir() || !strings.HasSuffix(fi.Name(), ".c") || strings.Contains(fi.Name(), "_cgo_export.c") || strings.Contains(fi.Name(), "_cgo_main.c") {
				continue
			}
			fi_abs_path := abs_path + "/" + fi.Name()
			unifdef_fi_abs_path := abs_path + "/" + "unifdef_" + fi.Name()
			tmp_args := append(DSymbols, fi_abs_path, "-o", unifdef_fi_abs_path)
			unidef_cmd := exec.Command("unifdef", tmp_args...)
			logf(unidef_cmd.String())
			if b, err := unidef_cmd.CombinedOutput(); err != nil {
				logf(string(b))
				return nil
			} else {
				logf("succeed")
			}
			tmp_args = append(bit_code_args, "-o", absBuildPath+"/"+fi.Name()+".bc")
			tmp_args = append(tmp_args, unifdef_fi_abs_path)
			bit_code_cmd := exec.Command("clang-10", tmp_args...)
			logf(bit_code_cmd.String())
			if b, err := bit_code_cmd.CombinedOutput(); err != nil {
				logf(string(b))
				return nil
			} else {
				logf("succeed")
			}
			if err := os.Remove(unifdef_fi_abs_path); err != nil {
				logf("remove %s failed", unifdef_fi_abs_path)
				return err
			}
		}
		return nil
	}
	return filepath.WalkDir(*c_root_path, gen_unifdef_bc_fn)
}

///MYCODE
//	copy file to build/
func copyFileToBuild(src_path string, dst_path string) error {
	b, err := os.ReadFile(src_path)
	if err != nil {
		logf("read %s fail", src_path)
		return err
	}
	err = os.WriteFile(dst_path, b, 0644)
	if err != nil {
		logf("copy %s fail", dst_path)
	}
	return nil
}

///MYCODE
//	generate dot callgraph using bitcode files in build/
func bc2dot() error {
	link_args := []string{"-S"}
	rds, _ := ioutil.ReadDir(buildPathStr)
	for _, fi := range rds {
		link_args = append(link_args, absBuildPath+"/"+fi.Name())
	}
	link_args = append(link_args, "-o", "tmp.ll")
	link_cmd := exec.Command("llvm-link-10", link_args...)
	logf(link_cmd.String())
	if b, err := link_cmd.CombinedOutput(); err != nil {
		logf(string(b))
		return err
	}
	dot_args := []string{"-analyze", "-dot-callgraph", "tmp.ll"}
	dot_cmd := exec.Command("opt-10", dot_args...)
	logf(dot_cmd.String())
	if b, err := dot_cmd.CombinedOutput(); err != nil {
		logf(string(b))
		return err
	}
	return nil
}

///MYCODE
//	read c_dot_path callgraph
func getCGOdotGraphBytes() []byte {
	outbyte, _ := ioutil.ReadFile(*c_dot_path)
	return outbyte
}

///MYCODE
//	first find node in C nodes, then find node in Go nodes
//	return node whose name matches fn, nil instead.
func findNode(fn string, dotg *dotGraph) *dotNode {
	nodes := dotg.Nodes
	for _, node := range nodes {
		if node.ID == fn {
			return node
		}
	}
	for _, node := range dotg.Cluster.Nodes {
		elems := strings.Split(node.ID, ".")
		real_fn := elems[len(elems)-1]
		if real_fn == fn {
			return node
		}
	}
	for _, pkg_cluster := range dotg.Cluster.Clusters {
		for _, node := range pkg_cluster.Nodes {
			elems := strings.Split(node.ID, ".")
			real_fn := elems[len(elems)-1]
			if real_fn == fn {
				return node
			}
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
	logf("go2c map: %v\n", go2c)
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
	label := c_node.Get("label")
	if len(label) <= 1 {
		return ""
	}
	return trim2Brace(label)
}

///MYCODE
func addCGOdotGraph(prog *ssa.Program, dotg *dotGraph) *dotGraph {
	go2c := getGO2Cmap(prog)
	c_graph, err := graphviz.ParseBytes(getCGOdotGraphBytes())
	if err != nil {
		log.Fatal("graphviz.ParseBytes error")
	}
	logf("\n--------------------\nget CGO callgraph bytes\n--------------------")
	c_edges_num := c_graph.NumberEdges()
	c_node := c_graph.FirstNode()
	c_nodes_num := c_graph.NumberNodes()
	logf("nodenum %d", c_nodes_num)
	logf("edgenum %d", c_edges_num)
	nodes_map := make(map[string]*dotNode)
	logf("\n----------------\nget C's callgraph nodes\n----------------\n")
	for c_node != nil {
		//	TODO出现nextNode后panic: runtime error: invalid memory address or nil pointer dereference
		c_fn_str := getCFuncName(c_node)
		if c_fn_str == "" {
			break
		}

		var node *dotNode
		if node = findNode(c_fn_str, dotg); node == nil {
			logf("%s in c side", c_fn_str)
			node = defaultNode(c_fn_str)
			dotg.Nodes = append(dotg.Nodes, node)
		} else {
			logf("%s in go side", c_fn_str)
		}
		nodes_map[c_fn_str] = node
		c_node = c_graph.NextNode(c_node)
	}
	logf("\n--------------------\nadd C edges\n--------------------\n")
	c_node = c_graph.FirstNode()
	for c_node != nil {
		c_fn_str := getCFuncName(c_node)
		if c_fn_str == "" {
			break
		}
		caller := nodes_map[c_fn_str]
		out_edge := c_graph.FirstOut(c_node)
		for out_edge != nil {
			out_fn_str := getCFuncName(out_edge.Node())
			if out_fn_str == "" {
				break
			}
			callee := nodes_map[out_fn_str]
			dotg.Edges = append(dotg.Edges, defaultEdge(caller, callee))
			logf("add C's edge: %s -> %s", caller.ID, callee.ID)
			out_edge = c_graph.NextOut(out_edge)
		}
		c_node = c_graph.NextNode(c_node)
	}
	logf("\n-----------------\nadd Go2C edges\n-----------------\n")
	for _Cfunc_XXX, XXX := range go2c {
		caller := findNode(_Cfunc_XXX, dotg)
		if caller == nil {
			logf("go side %s()'s node not found", _Cfunc_XXX)
			continue
		}
		callee, ok := nodes_map[XXX]
		if !ok {
			logf("%s not found in C side", XXX)
			callee = defaultNode(XXX)
			dotg.Nodes = append(dotg.Nodes, callee)
		}
		edge := defaultEdge(caller, callee)
		dotg.Edges = append(dotg.Edges, edge)
	}
	return dotg
}
