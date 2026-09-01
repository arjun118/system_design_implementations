# building a trie with 3 mil queries - memory ooo

## premise

with my then code - not lazy allocation of children and storing caching words at every node = reaching out of memory

## gc

1. garbage collector's job - automated memory management system - find the memory alocated on the heap that the program will never use again and reclaim it so the operating system or runtime can reuse it
2. Another term for automatically recycling memory is garbage collection. At a high level, a garbage collector (or GC, for short) is a system that recycles memory on behalf of the application by identifying which parts of memory are no longer needed.
3. the runtime library includes the gc

## heap vs stack

1. stack -> lifo, structured, fixed size, stored vars are automatically popped off when a fn returns, no gc needed
2. heap -> dynamic , unstructured, long-lived, compiled cnanot determine the lifetime or size of an object at compile time (- dynamic slices, pointers returned from fns and large tree/tries)

## what doesnot need gc management

1. non pointer go value,stored in local vars (lexical scope in which its created)
2. basically "stack allocation" - space is stored on a goroutine stack
3. Go values whose memory cannot be allocated this way, because the Go compiler cannot determine its lifetime, are said to escape to the heap.
4. "The heap" can be thought of as a catch-all for memory allocation, for when Go values need to be placed somewhere. The act of allocating memory on the heap is typically referred to as "dynamic memory allocation" because both the compiler and the runtime can make very few assumptions as to how this memory is used and when it can be cleaned up. That's where a GC comes in: it's a system that specifically identifies and cleans up dynamic memory allocations.

## why does a go values escape to the heap

1. compiler cnanot determine the lifetime or size of an object at compile time (- dynamic slices, pointers returned from fns and large tree/tries)
2. Consider for instance the backing array of a slice whose initial size is determined by a variable, rather than a constant. Note that escaping to the heap must also be transitive: if a reference to a Go value is written into another Go value that has already been determined to escape, that value must also escape.
3. Whether a Go value escapes or not is a function of the context in which it is used and the Go compiler's escape analysis algorithm. It would be fragile and difficult to try to enumerate precisely when values escape: the algorithm itself is fairly sophisticated and changes between Go releases. For more details on how to identify which values escape and which do not, see the section on eliminating heap allocations.

## what is "dead"

1. tracing garbage collection
2. identification of in-use or live object
3. how? 4. object -> An object is a dynamically allocated piece of memory that contains one or more Go values. 5. Pointer—A memory address that references any value within an object. This naturally includes Go values of the form \*T, but also includes parts of built-in Go values. Strings, slices, channels, maps, and interface values all contain memory addresses that the GC must trace.
4. objects and pointers to other objects form the `object graph`
5. if an object is reachable - then its not dead
6. how to define reachability 9. scanning: To identify live memory, the GC walks the object graph starting at the program's roots, pointers that identify objects that are definitely in-use by the program. Two examples of roots are local variables and global variables. The process of walking the object graph is referred to as scanning. Another phrase you might see in the Go documentation is whether an object is reachable, which just means that the object can be discovered by the scanning process. Note also that, with one exception, once memory becomes unreachable, it stays unreachable.
7. keeping track of scanning (progress): 11. Go's GC uses the mark-sweep technique 12. `marking`: 13. in order to keep track of its progress, the GC also marks the values it encounters as live. 14. `sweeping`: Once tracing is complete, the GC then walks over all memory in the heap and makes all memory that is not marked available for allocation. This process is called sweeping.

## GC cycle

1. go's gc has two phases -`mark phase` and `sweep phase`
2. `its not possible to release memory back ot be allocated until all memory has been traced`
3. seperate acts : `marking` and `sweeping`
4. if there is not gc-related work - gc will be in a `inactive phase`
5. so three phases - `mark`, `sweep`, `inactive`
6. gc continuously rotates through these three phases of sweeping , ooff and marking in - whats known as the gc cycle
7. For the purposes of this document, consider the GC cycle starting with sweeping, turning off, then marking.

## costs

1. gc involves 2 resources 2. memory 3. cpu time

### memory costs

1. live heap memory + new heap memory allocated before mark phase and space for metadata

gc memory cost for cycle n = live heap from cycle n+1 + new heap

live heap memory = determined to be live by the previous gc cycle
new heap memory = any memory allocated in the current cycle [may or may not live by the end]

### cpu costs

1. these are modelled ad fixed cost per cycle + a marginal cost that scales proportionally wiht size of the live heap (incurred while scanning memory )

gc cput time for cycle N = fixed cpu time + average cpu time cost per byte \* live heap mmeory found in cycle N

The fixed CPU time cost per cycle includes things that happen a constant number of times each cycle, like initializing data structures for the next GC cycle. This cost is typically small, and is included just for completeness.

This model ignores sweeping costs, which are proportional to total heap memory, including memory that is dead (it must be made available for allocation). For Go's current GC implementation, sweeping is so much faster than marking and scanning that the cost is negligible in comparison. - why the sweeping cost is neglible in go gc implementation? and why is it much faster

total CPU cost of the garbage collector depends on the total number of GC cycles in a given time frame.

## Steady stage of an application

in gc's POV steady state of an application is

1. ratte at which the application allcoated new mmeory (in bytes/s) is constant
2. marginal costs of gc are constant - This means that statistics of the object graph, such as the distribution of object sizes, the number of pointers, and the average depth of data structures, remain the same from cycle to cycle.

simple - if we execute gc more frequently when we use less memory and vice versa.

deciding when the gc should start is the main parameter which the user has control over - GOGC at a high level determines the trade-off between gc cpu and memory

## GOGC

1. working: works by determining the target heap size after each cycle that is a target value for total heap size in the next cycle
2. goal: finish colletcion cycle before the total heap size exceeds the target heap size
3. total heap size = live heap size at the end of the previous cycel + any new heap memory allocated by the aplication since the previous cycel

Target heap memory = Live heap + (Live heap + GC roots) \* GOGC / 100

gc roots = starting points that the garbage collcetor uses to discover all active , in-use memory - inlcudes goroutine stakcs + global + package level variables etc..

`heap target controls the gc frequency - the bigger the target, the longer the gc can wait to start another mark phase and vice versa`

```
doubling GOGC will double heap memory overheads and roughly halve GC CPU cost, and vice versa.
```

## Memory limit

GOGC doensot take into account that the memory is finite

1. this sets a max on the total amount of memory that the go runtime can use.

```
by setting a memory limit and turning up GOGC, we can get the best of both worlds: no memory limit breach, and better resource economy.
```

For this reason, the memory limit is defined to be soft. The Go runtime makes no guarantees that it will maintain this memory limit under all circumstances; it only promises some reasonable amount of effort. This relaxation of the memory limit is critical to avoiding thrashing behavior, because it gives the GC a way out: let memory use surpass the limit to avoid spending too much time in the GC.

This situation, where the program fails to make reasonable progress due to constant GC cycles, is called thrashing.
