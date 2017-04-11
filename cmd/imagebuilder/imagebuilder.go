package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	dockertypes "github.com/docker/engine-api/types"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/containers/storage/storage"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerclient"
	"github.com/projectatomic/buildah/imagebuildah"
)

type buildDriver interface {
	Prepare(*imagebuilder.Builder, *parser.Node, string) error
	Execute(*imagebuilder.Builder, *parser.Node) error
	Commit(*imagebuilder.Builder) error
}

func main() {
	if imagebuildah.InitReexec() {
		return
	}

	log.SetFlags(0)
	options := dockerclient.NewClientExecutor(nil)
	var tags stringSliceFlag
	var dockerfilePath string
	var imageFrom string
	var mountSpecs stringSliceFlag
	var forceDirect bool
	var storeGraphRoot, storeRunRoot, storeGraphDriverName string
	var storeGraphDriverOptions stringSliceFlag

	flag.Var(&tags, "t", "The name to assign this image, if any. May be specified multiple times.")
	flag.Var(&tags, "tag", "The name to assign this image, if any. May be specified multiple times.")
	flag.StringVar(&dockerfilePath, "f", dockerfilePath, "An optional path to a Dockerfile to use. You may pass multiple docker files using the operating system delimiter.")
	flag.StringVar(&dockerfilePath, "file", dockerfilePath, "An optional path to a Dockerfile to use. You may pass multiple docker files using the operating system delimiter.")
	flag.StringVar(&imageFrom, "from", imageFrom, "An optional FROM to use instead of the one in the Dockerfile.")
	flag.Var(&mountSpecs, "mount", "An optional list of files and directories to mount during the build. Use SRC:DST syntax for each path.")
	flag.BoolVar(&options.AllowPull, "allow-pull", true, "Pull the images that are not present.")
	flag.BoolVar(&options.IgnoreUnrecognizedInstructions, "ignore-unrecognized-instructions", true, "If an unrecognized Docker instruction is encountered, warn but do not fail the build.")
	flag.BoolVar(&options.StrictVolumeOwnership, "strict-volume-ownership", false, "Due to limitations in docker `cp`, owner permissions on volumes are lost. This flag will fail builds that might fall victim to this.")
	flag.BoolVar(&forceDirect, "direct", false, "Force building using cri-o libraries instead of dockerclient.")
	flag.StringVar(&storeGraphRoot, "root", storage.DefaultStoreOptions.GraphRoot, "Root directory for storage driver when building with cri-o libraries.")
	flag.StringVar(&storeRunRoot, "runroot", storage.DefaultStoreOptions.RunRoot, "State directory for Storage when building with cri-o libraries.")
	flag.StringVar(&storeGraphDriverName, "storage-driver", storage.DefaultStoreOptions.GraphDriverName, "Storage driver to use when building with cri-o libraries.")
	flag.Var(&storeGraphDriverOptions, "storage-options", "Storage driver options to use when building with cri-o libraries.")

	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("You must provide one argument, the name of a directory to build")
	}

	options.Directory = args[0]
	if len(tags) > 0 {
		options.Tag = tags[0]
		options.AdditionalTags = tags[1:]
	}
	if len(dockerfilePath) == 0 {
		dockerfilePath = filepath.Join(options.Directory, "Dockerfile")
	}

	var mounts []dockerclient.Mount
	for _, s := range mountSpecs {
		segments := strings.Split(s, ":")
		if len(segments) != 2 {
			log.Fatalf("--mount must be of the form SOURCE:DEST")
		}
		mounts = append(mounts, dockerclient.Mount{SourcePath: segments[0], DestinationPath: segments[1]})
	}
	options.TransientMounts = mounts

	options.Out, options.ErrOut = os.Stdout, os.Stderr
	options.AuthFn = func(name string) ([]dockertypes.AuthConfig, bool) {
		return nil, false
	}
	options.LogFn = func(format string, args ...interface{}) {
		if glog.V(2) {
			log.Printf("Builder: "+format, args...)
		} else {
			fmt.Fprintf(options.ErrOut, "--> %s\n", fmt.Sprintf(format, args...))
		}
	}

	// Accept ARGS on the command line
	arguments := make(map[string]string)

	dockerfiles := filepath.SplitList(dockerfilePath)
	if len(dockerfiles) == 0 {
		dockerfiles = []string{filepath.Join(options.Directory, "Dockerfile")}
	}

	_, node, err := imagebuilder.NewBuilderForFile(dockerfiles[0], arguments)
	if err != nil {
		log.Fatalf(err.Error())
		return
	}

	var driver buildDriver
	direct := imagebuilder.SplitChildren(node, "DIRECT")
	if !forceDirect && len(direct) == 0 {
		if err := options.DefaultExcludes(); err != nil {
			log.Fatalf("error: Could not parse default .dockerignore: %v", err)
			return
		}

		client, err := docker.NewClientFromEnv()
		if err != nil {
			log.Fatalf("error: No connection to Docker available: %v", err)
			return
		}
		options.Client = client

		// TODO: handle signals
		defer func() {
			for _, err := range options.Release() {
				log.Fatalf("error: Unable to clean up build: %v\n", err)
			}
		}()

		driver = options
	} else {
		bPullPolicy := imagebuildah.PullNever
		if options.AllowPull {
			bPullPolicy = imagebuildah.PullIfMissing
		}
		boptions := imagebuildah.BuildOptions{
			ContextDirectory:               options.Directory,
			PullPolicy:                     bPullPolicy,
			IgnoreUnrecognizedInstructions: options.IgnoreUnrecognizedInstructions,
			Compression:                    imagebuildah.Gzip,
			Args:                           arguments,
			Output:                         options.Tag,
			Out:                            options.Out,
			Err:                            options.ErrOut,
			Log:                            options.LogFn,
			AdditionalTags:                 options.AdditionalTags,
			//StrictVolumeOwnership:        options.StrictVolumeOwnership,
		}
		for _, s := range mounts {
			m := imagebuildah.Mount{
				Source:      s.SourcePath,
				Destination: s.DestinationPath,
				Type:        "bind",
				Options:     []string{"bind"},
			}
			boptions.TransientMounts = append(boptions.TransientMounts, m)
		}

		storeOptions := storage.StoreOptions{
			GraphRoot:          storeGraphRoot,
			RunRoot:            storeRunRoot,
			GraphDriverName:    storeGraphDriverName,
			GraphDriverOptions: storeGraphDriverOptions,
		}
		store, err := storage.GetStore(storeOptions)
		if err != nil {
			log.Fatalf("error: Error initializing storage: %v", err)
			return
		}
		defer store.Shutdown(false)

		executor, err := imagebuildah.NewExecutor(store, boptions)
		if err != nil {
			log.Fatalf("error: Error setting up to build: %v", err)
			return
		}

		// TODO: handle signals
		defer func() {
			if err := executor.Delete(); err != nil {
				fmt.Fprintf(boptions.Err, "error: Unable to clean up build: %v\n", err)
			}
		}()

		driver = executor
	}

	if err := build(dockerfiles[0], dockerfiles[1:], arguments, imageFrom, driver); err != nil {
		log.Fatal(err.Error())
	}
}

func build(dockerfile string, additionalDockerfiles []string, arguments map[string]string, from string, e buildDriver) error {
	b, node, err := imagebuilder.NewBuilderForFile(dockerfile, arguments)
	if err != nil {
		return err
	}
	if err := e.Prepare(b, node, from); err != nil {
		return err
	}
	if err := e.Execute(b, node); err != nil {
		return err
	}

	for _, s := range additionalDockerfiles {
		_, node, err := imagebuilder.NewBuilderForFile(s, arguments)
		if err != nil {
			return err
		}
		if err := e.Execute(b, node); err != nil {
			return err
		}
	}

	return e.Commit(b)
}

type stringSliceFlag []string

func (f *stringSliceFlag) Set(s string) error {
	*f = append(*f, s)
	return nil
}

func (f *stringSliceFlag) String() string {
	return strings.Join(*f, " ")
}
