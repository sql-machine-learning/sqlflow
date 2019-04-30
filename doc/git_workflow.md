# Notes for Git Workflow in SQLFlow project
1. This doc is mainly written for new Git and Github user who is interested in SQLFlow. Below we are illustrating the workflow through a sequence of commands. 
1. For experienced contributors, this branching model is what we follow https://nvie.com/posts/a-successful-git-branching-model/.

Now let's look at this example:

## Create local repo
Get SQLFlow source code through git clone as well as other dependencies (if necessary)

## Define/add remote tracking branch
Rename origin to upstream after git clone the forked repo, then point origin to the actual repo https://github.com/sql-machine-learning/sqlflow. This link well explains fork operations on Github: https://help.github.com/en/articles/fork-a-repo

```
git remote rename origin upstream
git remote add origin https://github.com/sql-machine-learning/sqlflow/
```

## Fetch Origin
First let's run git fetch to download commits, files, and refs from a remote repository into your local machine.
```bash
git fetch origin develop
```

## Checkout Origin Develop
Checkout remote develop branch which contains the latest code and commits. This command make sure we start from the most updated state of the repo.
```bash
git checkout remotes/origin/develop
```

## Create new feature branch and make commits
This command creates new feature branch out of origin develop branch, which is the latest. Traditionally, we create an issue along with the pull request to make sure we can track every change to the codebase.
```bash
git checkout -b new_feature_branch_issue_000
```

Now you can make necessary changes, add to the repo and use git diff to check the differences before making commits. 
```bash
git commit -am "Add feature *"
```
This command commits the change and you can use git log to make sure the commit actually happened.

## Push to Upstream for stagging
This commands push commits in the feature branch new_feature_branch_issue_000 to remote upstream repo, which is the forked one in your personal github. This command will automatically create a branch in the repo as well. Note this branch we created locally and in the forked repo are only intended for this specific change. We can leave it as is or clean up later on.
```bash
git push upstream HEAD:new_feature_branch_issue_000
```

## Create pull request
Now we can go to github and create the pull request with some descriptions. 

## Clean up
Lastly, you may clean up local and remote feature branch using below command. Note, -D will force delete for unmerged changes in the local branch. 

```bash
git branch -D new_feature_branch_issue_000
git checkout develop
git pull
git branch -D new_feature_branch_issue_000
git push origin :new_feature_branch_issue_000
```
