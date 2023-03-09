package main

import (
	"fmt"

	"github.com/buildpacks/lifecycle/image"

	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/buildpacks/lifecycle"
	"github.com/buildpacks/lifecycle/auth"
	"github.com/buildpacks/lifecycle/buildpack"
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/buildpacks/lifecycle/cmd/lifecycle/cli"
	"github.com/buildpacks/lifecycle/platform"
	"github.com/buildpacks/lifecycle/priv"
)

type analyzeCmd struct {
	*platform.Platform

	docker   client.CommonAPIClient // construct if necessary before dropping privileges
	keychain authn.Keychain         // construct if necessary before dropping privileges
}

// DefineFlags defines the flags that are considered valid and reads their values (if provided).
func (a *analyzeCmd) DefineFlags() {
	if a.PlatformAPI.AtLeast("0.12") {
		cli.FlagLayoutDir(&a.LayoutDir)
		cli.FlagUseLayout(&a.UseLayout)
		cli.FlagRunPath(&a.RunPath)
	}
	if a.PlatformAPI.AtLeast("0.9") {
		cli.FlagLaunchCacheDir(&a.LaunchCacheDir)
		cli.FlagSkipLayers(&a.SkipLayers)
	}
	switch {
	case a.PlatformAPI.AtLeast("0.7"):
		cli.FlagAnalyzedPath(&a.AnalyzedPath)
		cli.FlagCacheImage(&a.CacheImageRef)
		cli.FlagGID(&a.GID)
		cli.FlagLayersDir(&a.LayersDir)
		cli.FlagPreviousImage(&a.PreviousImageRef)
		cli.FlagRunImage(&a.RunImageRef)
		cli.FlagStackPath(&a.StackPath)
		cli.FlagTags(&a.AdditionalTags)
		cli.FlagUID(&a.UID)
		cli.FlagUseDaemon(&a.UseDaemon)
	default:
		cli.FlagAnalyzedPath(&a.AnalyzedPath)
		cli.FlagCacheDir(&a.CacheDir)
		cli.FlagCacheImage(&a.CacheImageRef)
		cli.FlagGID(&a.GID)
		cli.FlagGroupPath(&a.GroupPath)
		cli.FlagLayersDir(&a.LayersDir)
		cli.FlagSkipLayers(&a.SkipLayers)
		cli.FlagUID(&a.UID)
		cli.FlagUseDaemon(&a.UseDaemon)
	}
}

// Args validates arguments and flags, and fills in default values.
func (a *analyzeCmd) Args(nargs int, args []string) error {
	if nargs != 1 {
		err := fmt.Errorf("received %d arguments, but expected 1", nargs)
		return cmd.FailErrCode(err, cmd.CodeForInvalidArgs, "parse arguments")
	}
	a.LifecycleInputs.OutputImageRef = args[0]
	if a.UseLayout {
		if err := platform.GuardExperimental(platform.LayoutFormat, cmd.DefaultLogger); err != nil {
			return err
		}
	}
	return nil
}

// Privileges validates the needed privileges.
func (a *analyzeCmd) Privileges() error {
	var err error
	a.keychain, err = auth.DefaultKeychain(a.RegistryImages()...)
	if err != nil {
		return cmd.FailErr(err, "resolve keychain")
	}
	if a.UseDaemon {
		a.docker, err = priv.DockerClient()
		if err != nil {
			return cmd.FailErr(err, "initialize docker client")
		}
	}
	if err = priv.EnsureOwner(a.UID, a.GID, a.LayersDir, a.CacheDir, a.LaunchCacheDir); err != nil {
		return cmd.FailErr(err, "chown volumes")
	}
	if err = priv.RunAs(a.UID, a.GID); err != nil {
		return cmd.FailErr(err, fmt.Sprintf("exec as user %d:%d", a.UID, a.GID))
	}
	return nil
}

// Exec executes the command.
func (a *analyzeCmd) Exec() error {
	imageHandler, err := image.NewHandler(image.HandlerOptions{UseDaemon: a.UseDaemon, UseLayout: a.UseLayout, DockerClient: a.docker, RegistryKeychain: a.keychain})
	if err != nil {
		return cmd.FailErr(err, "get images")
	}
	if err := platform.ResolveInputs(platform.Analyze, a.LifecycleInputs, imageHandler, cmd.DefaultLogger); err != nil {
		return cmd.FailErrCode(err, cmd.CodeForInvalidArgs, "resolve inputs")
	}
	factory := lifecycle.NewAnalyzerFactory(
		a.PlatformAPI,
		&cmd.BuildpackAPIVerifier{},
		NewCacheHandler(a.keychain),
		lifecycle.NewConfigHandler(),
		imageHandler,
		NewRegistryHandler(a.keychain),
	)
	analyzer, err := factory.NewAnalyzer(
		a.AdditionalTags,
		a.CacheImageRef,
		a.LaunchCacheDir,
		a.LayersDir,
		a.CacheDir,
		buildpack.Group{},
		a.GroupPath,
		a.OutputImageRef,
		a.PreviousImageRef,
		a.RunImageRef,
		a.SkipLayers,
		cmd.DefaultLogger,
	)
	if err != nil {
		return unwrapErrorFailWithMessage(err, "initialize analyzer")
	}
	analyzedMD, err := analyzer.Analyze()
	if err != nil {
		return cmd.FailErrCode(err, a.CodeFor(platform.AnalyzeError), "analyze")
	}
	if err = analyzedMD.WriteTOML(a.AnalyzedPath); err != nil {
		return cmd.FailErr(err, "write analyzed")
	}
	return nil
}
