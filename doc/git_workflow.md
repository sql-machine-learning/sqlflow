#Git Workflow for SQLFlow project
This doc is mainly written for new contributors in this project, those who are experienced open source developers can skip the doc. Please read carefully and leave comments as it needs and helps us improve as more people from the community are joing force. 

## Fetch Upstream
First let's run git fetch to download commits, files, and refs from a remote repository into your local repo
```bash
git fetch upstream develop
```

## Checkout Remote Develop for the latest code
Then we checkout remote develop branch which contains the latest code and commits. This command make sure we start from the most updated state of the repo
```bash
git checkout remotes/upstream/develop
```
## Create new feature branch and make commits
This command creates new feature branch out of remote develop branch, which is the latest. Conventially, we create an issue along with a pull request to make sure we can track every changes to the codebase. Please follow the convention as much as you can.
```bash
git checkout -b new_feature_branch_issue_000
```

Now you can make necessary changes to the repo and use git diff to check the differences before commit. 
```bash
git commit -am "Add feature *"
```
This command commits the change and you can use git log to make sure the commit actually happened in the local repo.

## Push to forked repo for stagging
This commands push commits in the feature branch new_feature_branch_issue_000 to remote origin, which is the forked repo in your personal github. This command will automatically create a branch in the repo as well. Note this branch we created locally and in the forked repo are only intended for this specific code change. We could leave it as is or clean up later on.
```bash
git push origin HEAD:new_feature_branch_issue_000
```

## Create pull request
Now we can go to github and create the pull request with some descriptions. 

## Clean up branches
Lastly, you may clean up local feature branch using below command. Note, -D will force delete for unmerged changes in the local branch. 

```bash
git branch -D new_feature_branch_issue_000
```