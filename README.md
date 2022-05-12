# bake

Declarative tasks orchestration

## target "picture"

- data tasks
  - fetch state information necessary to run the tasks
- list (public) tasks:
  - a task is public if it has a description
  - TODO: should I allow `description` to depend on local values?
- store a state file
  - with hashes of all targets
    - https://stackoverflow.com/a/1761615
  - TODO: with hashes of all sources? would this be too slow?
- dry-run a (public) task:
  - provides an overview of the tasks it would run
  - provides a diff of target changes
  - TODO: can a tasks' for_each and/or command depend on another task output? or only on data?
    - this probably decides the direction of 'bake':
      - a task automation tool would allow depending on the previous command
      - a (declarative) build automation tool would NOT allow it since it makes
        the build process not-transparent
- run a (public) task:
  - resolve all data and locals
  - run the tasks in dependency order
- prune targets:
  - removes all files created by any target 
- watch a (public) target:
  - run or dry-run a target task
- cache targets of a recipe
  - store all target results in a zip file
  - store all targets hashes in a state file

Definitions:
- target: a task which provides sources and creates arguments
- task: a target or phony instance
