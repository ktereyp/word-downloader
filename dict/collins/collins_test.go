package collins

import (
	"fmt"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	"io/ioutil"
	"os"
	"testing"
)

func TestCollinsDict_Lookup(t *testing.T) {
	const (
		// These paths will be different on your system.
		seleniumPath     = "selenium/selenium-server.jar"
		geckoDriverPath  = "selenium/geckodriver"
		chromeDriverPath = "selenium/chromedriver"
		port             = 8080
	)
	opts := []selenium.ServiceOption{
		//selenium.GeckoDriver(geckoDriverPath), // Specify the path to GeckoDriver in order to use Firefox.
		selenium.ChromeDriver(chromeDriverPath), // Specify the path to GeckoDriver in order to use Firefox.
		selenium.Output(os.Stderr),              // Output debug information to STDERR.
	}
	selenium.SetDebug(true)
	service, err := selenium.NewSeleniumService(seleniumPath, port, opts...)
	if err != nil {
		panic(err) // panic is used only as an example and is not otherwise recommended.
	}
	defer service.Stop()

	// Connect to the WebDriver instance running locally.
	//caps := selenium.Capabilities{"browserName": "firefox"}
	caps := selenium.Capabilities{"browserName": "chrome"}
	caps.AddChrome(chrome.Capabilities{
		Path: "selenium/chrome-linux/chrome",
	})
	caps.AddProxy(selenium.Proxy{
		Type: selenium.Manual,
		HTTP: "http://127.0.0.1:8888",
		SSL:  "http://127.0.0.1:8888",
	})
	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		panic(err)
	}
	defer wd.Quit()

	// Navigate to the simple playground interface.
	if err := wd.Get("https://www.collinsdictionary.com/dictionary/english/exhort"); err != nil {
		panic(err)
	}

	source, err := wd.PageSource()
	if err != nil {
		panic(err)
	}
	wd.Close()
	ioutil.WriteFile("my_test.html", []byte(source), 0644)
}

func TestCollinsDict_Lookup2(t *testing.T) {
	dict := NewDict()
	word, err := dict.Lookup("exhort")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("success: %v", word.DefinitionHtml(false))
}
