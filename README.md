# merge bot

the merge bot takes care of ensuring proper protocol is followed when working with
pull-requests.

specifically it…

… marks fresh PRs as WIP (yes, open them early!)  
… automatically rebases master to ensure PR is up 2 date  
… once LGTM'd it will rebase & wait for tests and merge automatically  

## Architecture:

one cache (git clone @master of repo)
using a channel to sychronize startup of rebase workers
rebase workers based off of work tree of git clone
one rebase worker (chan) per PR 
worker stops after inactivity period (7 days)
