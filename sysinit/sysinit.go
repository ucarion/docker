package sysinit

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/netlink"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Setup networking
func setupNetworking(gw string) {
	if gw == "" {
		return
	}

	ip := net.ParseIP(gw)
	if ip == nil {
		log.Fatalf("Unable to set up networking, %s is not a valid IP", gw)
		return
	}

	if err := netlink.AddDefaultGw(ip); err != nil {
		log.Fatalf("Unable to set up networking: %v", err)
	}
}

// Setup working directory
func setupWorkingDirectory(workdir string) {
	if workdir == "" {
		return
	}
	if err := syscall.Chdir(workdir); err != nil {
		log.Fatalf("Unable to change dir to %v: %v", workdir, err)
	}
}

// Takes care of dropping privileges to the desired user
func changeUser(u string) {
	if u == "" {
		return
	}
	userent, err := utils.UserLookup(u)
	if err != nil {
		log.Fatalf("Unable to find user %v: %v", u, err)
	}

	uid, err := strconv.Atoi(userent.Uid)
	if err != nil {
		log.Fatalf("Invalid uid: %v", userent.Uid)
	}
	gid, err := strconv.Atoi(userent.Gid)
	if err != nil {
		log.Fatalf("Invalid gid: %v", userent.Gid)
	}

	if err := syscall.Setgid(gid); err != nil {
		log.Fatalf("setgid failed: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		log.Fatalf("setuid failed: %v", err)
	}
}

// Clear environment pollution introduced by lxc-start
func cleanupEnv() {
	os.Clearenv()
	var lines []string
	content, err := ioutil.ReadFile("/.dockerenv")
	if err != nil {
		log.Fatalf("Unable to load environment variables: %v", err)
	}
	err = json.Unmarshal(content, &lines)
	if err != nil {
		log.Fatalf("Unable to unmarshal environment variables: %v", err)
	}
	for _, kv := range lines {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		os.Setenv(parts[0], parts[1])
	}
}

func executeProgram(name string, args []string) {
	path, err := exec.LookPath(name)
	if err != nil {
		log.Printf("Unable to locate %v", name)
		os.Exit(127)
	}

	if err := syscall.Exec(path, args, os.Environ()); err != nil {
		panic(err)
	}
}

// Sys Init code
// This code is run INSIDE the container and is responsible for setting
// up the environment before running the actual process
func SysInit() {
	if len(os.Args) <= 1 {
		fmt.Println("You should not invoke dockerinit manually")
		os.Exit(1)
	}
	var u = flag.String("u", "", "username or uid")
	var gw = flag.String("g", "", "gateway address")
	var workdir = flag.String("w", "", "workdir")

	flag.Parse()

	cleanupEnv()
	setupNetworking(*gw)
	setupWorkingDirectory(*workdir)
	changeUser(*u)
	executeProgram(flag.Arg(0), flag.Args())
}
