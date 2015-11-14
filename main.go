package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/docker/libcompose/docker"
	"github.com/docker/libcompose/project"
	dockerclient "github.com/fsouza/go-dockerclient"
)

var ignore bool

var nodes int

var services map[string]interface{}

var p *project.Project

// Looks at the environment variables for the service and returns the value of
// scale if it exists
func getScale(env project.MaporEqualSlice, nodes int) (int, error) {
	for _, e := range env.Slice() {
		v := strings.Split(e, "=")
		if v[0] == "scale" {
			if v[1] == "N" {
				return nodes, nil
			} else {
				scale, err := strconv.Atoi(v[1])
				return scale, err
			}
		}
	}
	return 1, nil
}

// Actually does the scaling of the service
func scale() {
	ignore = true
	for k, _ := range services {
		log.Printf("Scaling %s\n", k)
		foo, _ := p.CreateService(k)
		scale, _ := getScale(foo.Config().Environment, nodes)
		foo.Scale(scale)
	}
	// Something's not quite right here
	time.Sleep(5 * time.Second)
	log.Printf("Done Scaling\n")
	ignore = false
}

// Determines how many nodes are available in the swarm for use in getScale()
func getNodes(client *dockerclient.Client) int {
	var env []interface{}

	info, _ := client.Info()
	envs := info.Map()

	json.Unmarshal([]byte(envs["DriverStatus"]), &env)
	for _, e := range env {
		v := e.([]interface{})
		if v[0] == "\bNodes" {
			nodes, _ := strconv.Atoi(v[1].(string))
			return nodes
		}
	}
	return 0
}

// Watches for Docker events, ignoring ones when we're in the midst of scaling
// Triggers a scale operation if we're not
func watchEvents(events chan *dockerclient.APIEvents) {
	for {
		event := <-events
		if ignore {
			log.Printf("Docker Event: %s from %s, ignoring\n", event.Status, event.From)
		} else {
			if event.Status == "die" {
				log.Printf("Docker Event: %s from %s, Triggering scale check\n", event.Status, event.From)
				scale()
			} else {
				log.Printf("Docker Event: %s from %s, ignoring\n", event.Status, event.From)
			}
		}
	}
}

// main function
func main() {

	fmt.Println("Feed me a compose file now:")
	// Read in our compose file from stdin
	yamlbytes, err := ioutil.ReadAll(os.Stdin)

	// unmarshal it so we can enumerate our services
	yaml.Unmarshal(yamlbytes, &services)

	// create a new compose project
	p, err = docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeBytes: yamlbytes,
			ProjectName:  "my-compose", // TODO make an environment variable
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	// create our docker client link
	client, _ := dockerclient.NewClientFromEnv()

	// make and attach our listener channel
	events := make(chan *dockerclient.APIEvents)
	client.AddEventListener(events)

	// start watching for events
	go watchEvents(events)

	// main loop
	for {
		// look up how many nodes we have in the cluster
		// this is mainly for when a node is added
		nodes = getNodes(client)

		// Print the number of nodes we found
		log.Printf("Nodes: %d\n", nodes)

		// Do the heavy lifting once
		scale()

		// sleep for a bit, then check again
		time.Sleep(time.Minute) // TODO make an environment variable
	}
}
