# bake

Declarative tasks orchestration

## target "picture"

- ✅ data tasks
  - ✅ fetch state information necessary to run the tasks
- ✅ list (public) tasks:
  - ✅ a task is public if it has a description
- ✅ store a state file
  - TODO: with hashes of all sources? would this be too slow?
  - with hashes of all targets
    - https://stackoverflow.com/a/1761615
- ✅ dry-run a (public) task:
  - ✅ provides an overview of the tasks it would run
  - provides a diff of target changes
- ✅ run a (public) task:
  - ✅ resolve all data and locals
  - ✅ run the tasks in dependency order
- ✅ prune targets:
  - ✅ removes all files created by any target 
- watch a (public) target:
  - run or dry-run a target task
- cache targets of a recipe
  - store all target results in a zip file
  - store all targets hashes in a state file