## Repro steps

- Install containerd (v1.3.2 or later)
  - i.e. it must include commit [9abfc70](https://github.com/containerd/containerd/commit/9abfc700434c136e74d02eaaf1f1f4366c46f4cb)
- Install [ginkgo](https://github.com/onsi/ginkgo)
- Adjust the containerdSocket const in the test file if required
- `sudo -i`
- Ensure `go` and `ginkgo` are in your $PATH
- `ginkgo -untilItFails`

### Test results

After a few attempts (you might need to wait a minute or two), ginkgo will fail
with an error such as

```
Expected
    <[]error | len:1, cap:1>: [
        <*errors.errorString | 0xc00061e070>{
            s: "expected 'hi stdout', got: \"\"",
        },
    ]
to be empty
```

This indicates the echo process has terminated successfully with a zero exit
code, but its stdout has been lost.

### A working version of containerd

If you install a version of containerd prior to 9abfc70, e.g. v1.3.1 and rerun
the test, it will not fail.

