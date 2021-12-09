/*
 * Copyright 2021 ByteDance Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package decoder

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"unsafe"

	"github.com/bytedance/sonic/internal/jit"
	"github.com/bytedance/sonic/internal/rt"
	"github.com/twitchyliquid64/golang-asm/obj"
)


var (
    debugSyncGC  = os.Getenv("SONIC_SYNC_GC") == ""
    debugAsyncGC = os.Getenv("SONIC_NO_ASYNC_GC") == ""
)

var (
    _Instr_End _Instr = newInsOp(_OP_nil_1)

    _F_gc       = jit.Func(runtime.GC)
    _F_force_gc = jit.Func(debug.FreeOSMemory)
    _F_println  = jit.Func(println_wrapper)
    _F_print    = jit.Func(print)
    _F_checkptr = jit.Func(checkptr)
)

func println_wrapper(i int, op1 int, op2 int){
    println(i, " Intrs ", op1, _OpNames[op1], "next: ", op2, _OpNames[op2])
}

func print(i int){
    println(i)
}

func (self *_Assembler) force_gc() {
    self.save(_REG_go...)
    self.call(_F_gc)
    self.call(_F_force_gc)
    self.load(_REG_go...)
}

func (self *_Assembler) debug_instr(i int, v *_Instr) {
    if debugSyncGC {
        if (i+1 == len(self.p)) {
            self.print_gc(i, v, &_Instr_End) 
        } else {
            next := &(self.p[i+1])
            self.print_gc(i, v, next)
            name := _OpNames[next.op()]
            if strings.Contains(name, "save") {
                return
            }
        }
        self.force_gc()
    }
}

//go:noescape
//go:linkname checkptrAlignment runtime.checkptrAlignment
func checkptrAlignment(p unsafe.Pointer, elem *rt.GoType, n uintptr)

//go:noescape
//go:linkname checkptrArithmetic runtime.checkptrArithmetic
func checkptrArithmetic(p unsafe.Pointer, originals []unsafe.Pointer)

//go:noescape
//go:linkname checkptrBase runtime.checkptrBase
func checkptrBase(p unsafe.Pointer) uintptr

func checkptr(ptr uintptr) {
    fmt.Printf("ptr: %x\n", ptr)
    f := checkptrBase(unsafe.Pointer(uintptr(ptr)))
    if f == 0 {
        panic(fmt.Sprintf("invalid pointer: %v", ptr))
    } else if f == 1 {
        panic(fmt.Sprintf("stack pointer: %v", ptr))
    }
    fmt.Printf("ptr base: %x\n", f)
}

var _REG_checkptr = []obj.Addr {
    _ST,
    _VP,
    _IP,
    _IL,
    _IC,
    _AX,
    _DI,
    _R10,
}

func (self *_Assembler) check_ptr(ptr obj.Addr, lea bool) {
    self.save(_REG_checkptr...)
    if lea {
        self.Emit("LEAQ", ptr, _R10)
    } else {
        self.Emit("MOVQ", ptr, _R10)
    }
    self.Emit("MOVQ", _R10, jit.Ptr(_SP, 0))
    self.Emit("MOVQ", _F_checkptr, _R10)
    self.Rjmp("CALL", _R10)  
    self.load(_REG_checkptr...)
}