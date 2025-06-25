# Concurrent Janus

This is a repository for "Concurrent Janus: A Concurrent Extension of
Reversible Imperative Programming Language", submitted for MPLR2025.

## Contents

- Directory `compiler` contains Concurrent Janus to CRIL compiler
- Directory `progs` contains some sample programs written in Concurrent Janus
- Directory `runtime` contains CRIL interpreter
- Directory `util` contains (very minimal) utilities for use in both compiler and runtime

## Usage

### Compiler

```
$ go run ./compiler [-o output_file] <input_file>
```
This will read the input Concurrent Janus program, compile it to CRIL, and output to specified file. If output file is not specified (-o option is omitted), defaults to "a.crl".



### Runtime

```
$ go run ./runtime [options] <input_file>
```
This will start interactive runtime session for input CRIL program. Forward execution is done automatically on startup. Once that's done, user may give following commands from stdin.

- `fwd` sets the execution direction to forward
- `bwd` sets the execution direction to backward
- `run` will start the execution on specified direction (from current variable/Annotation DAG state). *Program will break upon backward execution from initial state / forward execution from final state.*
- `var` will display contents of heap memory. Global variables are stored here in declearation order.
- `proc` will display each process' id and its program counter.
- `dag` will display Annotation DAG in text format
- `deldag` will reset Annotation DAG. Next execution will have diffrent execution order, and result in reaching different final state (if program output depends on parallel execution order) in forward / not returning to initial state in backward

`Ctrl+C` will terminate runtime.

Command-line options (most useful ones)
- `-s <duration>` Add sleep of specified amount after every instruction. Higher values will result in more alternating execution order
- `-v` Enable verbose mode. Will output annotation DAG changes, variable read/write/lock/unlocks, push/pops, and V/P waits
- `-e` Debug feature. If enabled, record instructions. Execution will fail if block executed backwards does not match corresponding forward one
- `-t` If enabled, execution time is measured and displayed when done. Timer starts when root process is spawned, thus does not include initization (such as regex compiling)
- `--silent` Disables printing executed blocks. Will still print message indicating program is finished
