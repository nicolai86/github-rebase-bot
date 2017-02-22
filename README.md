# merge bot

the merge bot takes care of ensuring proper protocol is followed when working with
pull-requests.

specifically it…

… marks fresh PRs as WIP (yes, open them early!)  
… automatically rebases master to ensure PR is up 2 date  
… once LGTM'd it will rebase & wait for tests and merge automatically  
