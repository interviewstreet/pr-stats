# pr-stats
---
Various statistics on the pull requests in your repo. Basically it helps to analyze the duration between your pull request created at to first comment on that PR(m days). And from first comment to closed/merged at duration periode(n days).
It pushes your data to the elastic instance. With help of Kibana you can plot the graph of it and can analyze it.

Install
---
### OS X

- ensure you have mercurial installed as it is required for a dependency
    - using the homebrew package manager `brew install hg` [Homebrew Guide/Install](http://brew.sh/)
- install the Mac OS X binary build from https://golang.org/dl/
- follow instructions on http://golang.org/doc/install
- VERY IMPORTANT: Go has a required directory structure which the GOPATH needs to point to. Instructions can be found on http://golang.org/doc/code.html or by typing `go help gopath` in terminal.
- setup the directory structure in $GOPATH
    - `cd $GOPATH; mkdir src pkg bin`
    - create the github.com path and interviewstreet `mkdir -p src/github.com/interviewstreet; cd src/github.com/interviewstreet`
    - clone pr-stats `git clone https://github.com/interviewstreet/pr-stats.git; cd pr-stats`
    - make sure godep is installed, `go get github.com/tools/godep`
    - run `godep restore` to get all the dependencies as specified in `Godeps.json`
    - now you can build with `godep go build -a ./cmd/...`

 You may need to add $GOPATH to your PATH environment variable. Something along the lines of `export PATH="$GOPATH/bin:$PATH"` should work.

Configure
---------
There is a sample config in config.yaml.  The config defines the User, Repo, duration and url of the elastic instance.

```
AccessToken: <Github access token>
Search:
    User: interviewstreet
    Repo: hackerrank
TimeLine:
    Start: 2016-04-13
    End: 2016-04-25
```


Run
---------
```
go build main.go
go run main.go
```
The above commands store your data into .json file. Use Logstash to store your data in ES.

Logstash Config
---------
Configure your logstash and store the data in ES instant by following command.
```
logstash -f logstash-filter.conf < github.json
```

## License

This software is available under [MIT license](https://github.com/interviewstreet/pr-stats/blob/master/LICENSE).

