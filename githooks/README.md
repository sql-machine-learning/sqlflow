# Use Git Hooks

## gometalinter-pre-commit
1 Install  
`go get -u github.com/alecthomas/gometalinter && gometalinter --install`    

then `cd sqlflow`

2 Set hooksPath   
`git config core.hooksPath githooks`    
git --version > 2.9

3 Executable     
`chmod +x githooks/pre-commit`
