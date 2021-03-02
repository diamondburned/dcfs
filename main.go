package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/diamondburned/arikawa/v2/session"
	"github.com/diamondburned/arikawa/v2/state"
	"github.com/diamondburned/arikawa/v2/state/store/defaultstore"
	"github.com/diamondburned/arikawa/v2/utils/wsutil"
	"github.com/joho/godotenv"
)

func init() {
	// Ignore terminal loss // Useful for daemonization
	signal.Ignore(syscall.SIGHUP)
}

func main() {
	wsutil.WSDebug = func(v ...interface{}) {
		log.Println(v...)
	}

	cfgPath := flag.String("c", "", "Path to the config (.env) file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [flags...] mountpoint [format...]\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if *cfgPath != "" {
		if err := godotenv.Load(*cfgPath); err != nil {
			log.Fatalln("Can't parse config file at", *cfgPath)
		}
	}

	var (
		token      = os.Getenv("TOKEN") //""
		username   = os.Getenv("USERNAME")
		password   = os.Getenv("PASSWORD")
		mountpoint = flag.Arg(0)
	)

	if mountpoint == "" {
		flag.Usage()
		os.Exit(2)
	}

	var ses *session.Session
	var err error

	switch {
	case token != "":
		ses, err = session.New(token)
	case username != "" && password != "":
		ses, err = session.Login(username, password, "")
	default:
		log.Fatalln("No token or username and password given.")
	}

	if err != nil {
		log.Fatalln("Failed to authenticate:", err)
	}

	s := state.NewFromSession(ses, defaultstore.New())
	/*
		NewFromSession never returns an error
		> https://github.com/diamondburned/arikawa/blob/6c3becbdc5ef1a6032889be260b2c5d4313e6246/state/state.go#L123
	*/
	log.Println("Created a session. Logging in.")

	if err := s.Open(); err != nil {
		log.Fatalln("Failed to open a Discord connection:", err)
	}
	defer s.CloseGracefully()

	log.Println("Connected.")

	FS, err := NewFS(s)
	if err != nil {
		log.Fatalln("Failed to create a filesystem:", err)
	}

	if args := flag.Args(); len(args) > 2 {
		if err := FS.Fmt.ChangeMessageTemplate(args[1:]); err != nil {
			log.Fatalln("Failed to change message template:", err)
		}
	}

	log.Println("Created a filesystem")

	// Unmount before mounting, just in case.
	if fuse.Unmount(mountpoint) == nil {
		log.Println("Unmounted")
	}
	c, err := fuse.Mount(mountpoint)
	if err != nil {
		log.Fatalln("Failed to mount FUSE:", err)
	}

	log.Println("Mount point created at", mountpoint)

	u, err := s.Me()
	if err != nil {
		log.Fatalln("Failed to get myself:", err)
	}

	log.Println("Serving for user", u.Username)

	if err := fs.Serve(c, FS); err != nil {
		log.Fatalln("Failed to serve filesystem:", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	select {
	case <-c.Ready:
		if err := c.MountError; err != nil {
			log.Fatalln("Mount error:", err)
		}

	case <-sigs:
		log.Println("Ctrl+C pressed in Terminal or SIGTERM received.")
	}

	if err := fuse.Unmount(flag.Arg(0)); err != nil {
		log.Fatalln("Failed to unmount on close:", err)
	}

	log.Println("Unmounted")
}
