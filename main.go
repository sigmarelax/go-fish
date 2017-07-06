package main

import (
	"os"
	"path"
	"path/filepath"
	"plugin"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/patrobinson/go-fish/input"
	"github.com/patrobinson/go-fish/output"
)

// Rule is an interface for rule implementations
type Rule interface {
	Start(*chan interface{}, *chan interface{}, *sync.WaitGroup)
	Process(interface{}) bool
	String() string
}

// Input is an interface for input implemenations
type Input interface {
	Retrieve(*chan []byte)
	Init() error
}

// Output is an interface for output implementations
type Output interface {
	Sink(*chan interface{}, *sync.WaitGroup)
}

func main() {
	configFile := os.Args[1]
	file, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("Failed to open Config File: %v", err)
	}
	config, err := parseConfig(file)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var in interface{}
	if config.Input == "Kinesis" {
		in = &input.KinesisInput{
			StreamName: (*config.KinesisConfig).StreamName,
		}
	} else if config.Input == "File" {
		in = &input.FileInput{FileName: (*config.FileConfig).InputFile}
	} else {
		log.Fatalf("Invalid input type: %v", config.Input)
	}

	out := &output.FileOutput{FileName: (*config.FileConfig).OutputFile}

	run(config.RuleFolder, config.EventTypeFolder, in, out)
}

func run(rulesFolder string, eventFolder string, in interface{}, out interface{}) {
	input := in.(Input)
	output := out.(Output)

	err := input.Init()
	if err != nil {
		log.Fatalf("Input setup failed: %v", err)
	}
	var outWg sync.WaitGroup
	var ruleWg sync.WaitGroup

	outChan := startOutput(&output, &outWg)
	rChans := startRules(rulesFolder, outChan, &ruleWg)
	inChan := startInput(&input)
	eventTypes, err := getEventTypes(eventFolder)
	if err != nil {
		log.Fatalf("Failed to get Event plugins: %v", err)
	}

	// receive from inputs and send to all rules
	func(iChan *chan []byte, ruleChans []*chan interface{}) {
		for data := range *iChan {
			evt, err := matchEventType(eventTypes, data)
			if err != nil {
				log.Infof("Error matching event: %v", err)
			}
			for _, i := range ruleChans {
				*i <- evt
			}
		}
	}(inChan, rChans)

	log.Debug("Input done, closing rule channels\n")

	for _, c := range rChans {
		close(*c)
	}
	ruleWg.Wait()

	log.Debug("Closing output channels\n")
	close(*outChan)
	outWg.Wait()
}

func startOutput(out *Output, wg *sync.WaitGroup) *chan interface{} {
	(*wg).Add(1)
	outChan := make(chan interface{})
	go (*out).Sink(&outChan, wg)
	return &outChan
}

func startInput(in *Input) *chan []byte {
	inChan := make(chan []byte)
	go (*in).Retrieve(&inChan)
	return &inChan
}

func startRules(rulesFolder string, output *chan interface{}, wg *sync.WaitGroup) []*chan interface{} {
	pluginGlob := path.Join(rulesFolder, "/*.so")
	plugins, err := filepath.Glob(pluginGlob)
	if err != nil {
		log.Fatal(err)
	}

	var rules []*plugin.Plugin
	for _, pFile := range plugins {
		if plug, err := plugin.Open(pFile); err == nil {
			rules = append(rules, plug)
		}
	}

	log.Infof("Found %v rules", len(rules))

	var inputs []*chan interface{}
	for _, r := range rules {
		symRule, err := r.Lookup("Rule")
		if err != nil {
			log.Errorf("Rule has no Rule symbol: %v", err)
			continue
		}
		var rule Rule
		rule, ok := symRule.(Rule)
		if !ok {
			log.Errorf("Rule is not a rule type. Does it implement the Process() function?")
			continue
		}
		input := make(chan interface{})
		inputs = append(inputs, &input)
		log.Debugf("Starting %v\n", rule.String())
		(*wg).Add(1)
		go rule.Start(&input, output, wg)
	}

	return inputs
}
