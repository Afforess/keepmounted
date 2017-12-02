package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"strings"
	"syscall"
	"time"
)

func main() {
	source := flag.String("source", "", "the source device")
	destPath := flag.String("target", "", "path to the target mount location")
	options := flag.String("options", "", "mount options")
	mountType := flag.String("type", "", "mount type")
	interval := flag.Int("interval", 60, "how often the mount is checked (in seconds)")

	flag.Parse()

	mustExist(source, "-source device must be specified")
	mustExist(destPath, "-target path must be specified")
	mustExist(mountType, "-type mount type must be specified")
	mustBeRoot()
	ensureDest(*destPath)

	ensureMount(*source, *destPath, *options, *mountType, time.Duration(*interval))

	awaitDeath()
}

func ensureMount(source, destPath, options, mountType string, interval time.Duration) {
	for {
		if isMountOkay(source, destPath) {
			time.Sleep(interval * time.Second)
			continue
		}
		if isMountPoint(source, destPath) && !unmountPath(source, destPath) {
			fmt.Println("unable to unmount path: " + destPath)
			// XXX: what else to do here but retry?
			time.Sleep(interval * time.Second)
			continue
		}
		if !mountPath(source, destPath, options, mountType) {
			fmt.Println("unable to mount path: " + destPath)
			// XXX: what else to do here but retry?
			time.Sleep(interval * time.Second)
			continue
		}
	}
}

func mustExist(opt *string, desc string) {
	if opt == nil || *opt == "" {
		fmt.Fprintln(os.Stderr, desc)
		os.Exit(1)
	}
}

func mustBeRoot() {
	user, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to lookup current user: "+err.Error())
		os.Exit(3)
	}
	if user.Name != "root" {
		fmt.Fprintln(os.Stderr, "keepmounted can only be executed as root!")
		os.Exit(3)
	}
}

func ensureDest(destPath string) {
	stat, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "error, expected target path to exist: "+destPath)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error, failed to read target path: "+err.Error())
		os.Exit(2)
	}
	if !stat.IsDir() {
		fmt.Fprintln(os.Stderr, "error, target path is not a dir!")
		os.Exit(2)
	}
}

func pathExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func deleteTestFile(path string) bool {
	err := os.Remove(path)
	if err != nil {
		fmt.Println(".keepmounted file (" + path + ") could not be deleted... is the filesystem in RO mode?")
		fmt.Fprintln(os.Stderr, ".keepmounted file ("+path+") could not be deleted: "+err.Error())
		return false
	}
	if pathExists(path) {
		fmt.Fprintln(os.Stderr, ".keepmounted file ("+path+") was reported as deleted by the os, but is still present!")
		return false
	}
	return true
}

func mountPath(source, destPath, options, mountType string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	args := []string{"-t", mountType}
	if options != "" {
		args = append(args, "-o", options)
	}
	args = append(args, source, destPath)
	cmd := exec.CommandContext(ctx, "/bin/mount", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintln(os.Stderr, "/bin/mount "+destPath+" returned "+err.Error())
		fmt.Fprintln(os.Stderr, "/bin/mount output: "+string(output))
		return false
	}
	return isMountPoint(source, destPath)
}

func unmountPath(source, destPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/umount", destPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintln(os.Stderr, "/bin/umount "+destPath+" returned "+err.Error())
		fmt.Fprintln(os.Stderr, "/bin/umount output: "+string(output))
		return false
	}
	return !isMountPoint(source, destPath)
}

func isMountOkay(source, destPath string) bool {
	_, err := os.Stat(destPath)
	if err != nil {
		fmt.Println("mount dest path could not be stated: " + err.Error())
		return false
	}
	if !isMountPoint(source, destPath) {
		fmt.Println("mount point is not active")
		return false
	}
	keepMounted := path.Join(destPath, ".keepmounted")
	if pathExists(keepMounted) {
		fmt.Println(".keepmounted unexpectedly present, cleaning up: " + keepMounted)
		if !deleteTestFile(keepMounted) {
			return false
		}
	}
	file, err := os.Create(keepMounted)
	if err != nil {
		fmt.Println(".keepmounted file (" + keepMounted + ") could not be created!")
		fmt.Fprintln(os.Stderr, ".keepmounted file ("+keepMounted+") creation failed: "+err.Error())
		return false
	}
	file.Close()
	return deleteTestFile(keepMounted)
}

func awaitDeath() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		s := <-signalChan
		fmt.Println("received shutdown signal: " + s.String())
		os.Exit(0)
	}()
}

func isMountPoint(source, path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/mount")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintln(os.Stderr, "/bin/mount returned "+err.Error())
		fmt.Fprintln(os.Stderr, "/bin/mount output: "+string(output))
		return false
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, source) && strings.Contains(line, path) {
			return true
		}
	}
	return false
}
