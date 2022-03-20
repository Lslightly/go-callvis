# 过程

1. 遍历所有文件夹dirs
   1. 取定文件夹dir,对dir下每个go文件使用`go tool cgo x.go`生成_obj文件夹，如果有错，则跳过。
   2. 对于`dir/_obj`文件夹下所有`.c`文件（除去_cgo_export.c,_cgo_main.c）以及`dir`文件夹下所有.c文件使用`unifdef -D=A -U=B x.c -o unifdef_x.c`和`clang -c -emit-llvm -o path-to-build/x.c.bc _obj/unifdef_x.c`生成bitcode文件，对于_obw文件夹下的文件，clang添加`-I path-to-obj/../ -I path-to-obj/`选项
2. 对于path-to-build文件夹下所有bc文件进行链接，使用`llvm-link -S x1.bc x2.bc -o tmp.ll`，然后用`opt -analyze -dot-callgraph tmp.ll`生成callgraph.dot文件，然后就可以使用`go-callvis`生成桥接调用图。

TODO:

如果main pkg中的函数调用的其他文件夹下(也就是其他pkg)的函数g，则在调用图上会有到pkg.g的边，但是pkg.f进一步调用函数则不会被分析。