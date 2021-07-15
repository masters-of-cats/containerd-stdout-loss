package containerdstdout_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// This needs to be run as root so it can contact the containerd service
// Setting path, and maybe gopath is required before running ginkgo:
//
// export GOPATH=/home/vagrant/go
// export PATH=$PATH:/usr/local/go/bin/:$GOPATH/bin
// ginkgo -untilItFails

const containerdSocket = "/run/containerd/containerd.sock"

var _ = Describe("Stdouttest", func() {
	var (
		ctx       context.Context
		client    *containerd.Client
		image     containerd.Image
		container containerd.Container
		task      containerd.Task
	)

	BeforeEach(func() {
		var err error

		client, err = containerd.New(containerdSocket)
		Expect(err).NotTo(HaveOccurred())

		ctx = namespaces.WithNamespace(context.Background(), "example")

		image, err = client.Pull(ctx, "docker.io/library/busybox:latest", containerd.WithPullUnpack)
		Expect(err).NotTo(HaveOccurred())

		container, err = client.NewContainer(
			ctx,
			"example",
			containerd.WithNewSnapshot("busybox-snapshot", image),
			containerd.WithNewSpec(oci.WithImageConfig(image), oci.WithProcessArgs("sleep", "600")),
		)
		Expect(err).NotTo(HaveOccurred())

		task, err = container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
		Expect(err).NotTo(HaveOccurred())

		err = task.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		task.Delete(ctx, containerd.WithProcessKill)
		container.Delete(ctx, containerd.WithSnapshotCleanup)
		client.Close()
	})

	It("captures stdout consistently", func() {
		var wg sync.WaitGroup
		count := 10
		wg.Add(count)
		ec := make(chan error, count)
		for i := 0; i < count; i++ {
			go runProcess(ctx, i, &wg, ec, task)
		}

		wg.Wait()

		errs := []error{}
	outer:
		for {
			select {
			case err := <-ec:
				errs = append(errs, err)
			default:
				break outer
			}
		}

		Expect(errs).To(BeEmpty())
	})
})

func runProcess(ctx context.Context, i int, wg *sync.WaitGroup, ec chan error, task containerd.Task) {
	defer wg.Done()

	stdout := gbytes.NewBuffer()
	process, err := task.Exec(ctx, "say-hello-"+strconv.Itoa(i), &specs.Process{
		Args: []string{"/bin/echo", "hi stdout"},
		Cwd:  "/",
	}, cio.NewCreator(
		cio.WithStreams(nil, stdout, io.Discard),
	))
	if err != nil {
		ec <- err
		return
	}
	defer process.Delete(ctx, containerd.WithProcessKill)

	processExit, err := process.Wait(ctx)
	if err != nil {
		ec <- err
		return
	}

	err = process.Start(ctx)
	if err != nil {
		ec <- err
		return
	}

	exitStatus := <-processExit
	if exitStatus.Error() != nil {
		ec <- exitStatus.Error()
		return
	}

	if exitStatus.ExitCode() != 0 {
		ec <- fmt.Errorf("exit code: %d", exitStatus.ExitCode())
		return
	}

	if !bytes.Contains(stdout.Contents(), []byte("hi stdout")) {
		ec <- fmt.Errorf("expected 'hi stdout', got: %q", string(stdout.Contents()))
		return
	}
}
