package system

import (
	"log"
	"os/exec"
)

type Language struct{
	name string
	path string
	verison string
	
}

func GetLanguages()string{
	language, err := exec.Command("go", "version").Output()	
	if err != nil{
		log.Fatal(err)
	}
	
	// path, err := exec.LookPath()
	
	return string(language)
}



