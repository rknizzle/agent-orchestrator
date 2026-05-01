I want to build a gemini agent orchestration program in python. I want it to monitor my software project that I have hosted in Github. The tasks for my project are managed in 'github projects'. And its going to be based around statuses. 

The statuses will be something like this: But there might need to be more
- AI TODO
- AI IN PROGRESS
- AI FOLLOW UP QUESTIONS
- AI FOLLOW UP QUESTIONS ANSWERED
- AI READY TO PLAN
- AI PLANNING
- AI PLAN NEEDS REVIEW
- AI READY TO IMPLEMENT

And so the idea is that when you run the program you specify one of these types of states that it should look for. And it goes down the list in order and finds the first one with that status that its looking for. And then depending on the status theres a different gemini cli prompt that it calls. And after the prompt, the program will update the status and maybe write a comment or upload something.

Help me start to plan out some more specifics of this system and start laying out some specifics and we'll iterate

I think we can do AI INITIAL VIEWING instead of the more generic IN PROGRESS

Ok so heres the flow:
- I setup a ticket with AI TODO when I finish writing the specification of the work.
- The program gets this ticket and immediately sets it to INITIAL VIEWING so that another agent wouldnt pick it up at the same time.
- The gemini cli prompt will ready the ticket specification and either say its good and set it to AI READY TO PLAN or if it needs clarification it can post some questions to the ticket and set it to AI FOLLOW UP QUESTIONS
- I will view the follow up questions and answer them in a comment and set the status to AI FOLLOW UP QUESTIONS ANSWERED.
- Now another agent can look at this ticket again and read the answers and either decide it has more follow up questions and post the questions and set it back to AI FOLLOW UP QUESTIONS ANSWERED (this can happen as many times as needed) or it can set it to AI READY TO PLAN
- Now an agent can pick up the AI READY TO PLAN ticket. Itll plan it and post the plan to a comment and set the status to AI PLAN NEEDS REVIEW
- Ill review the plan and either set it to AI READY TO IMPLEMENT or if I have feedback ill post comments in reply to the plan and set it to AI READY TO PLAN.
- NOTE: I Think we might need another status here to say that the agent should read my plan feedback and update the plan and set it back to AI PLAN NEEDS REVIEW again. this can happen as many times as needed until it gets set to AI READY TO IMPLEMENT
- And then the agent can grab a ticket with AI READY TO IMPLEMENT and pull down the plan and build it out. It will make a PR after and link the PR in the ticket and set the status to AI PR READY.
- I will review the PR and either merge it in or post comments and set it to AI PR REVIEW FEEDBACK
- The agent will pick this up and look at the feedback and make changes and set the status back to PR READY. And then I can add more feedback and set it back to PR REVIEW FEEDBACK or merge it in


NOTE: I may not have specified all of them but there needs to be a status that the program sets the ticket to at each stage right after it looks at it to say that its being worked on so if another agent is running concurrently it wont pick up the same ticket. I think for now the simplest might just be to have one generic AI WORKING status that any of the phases can set to while the agnet is doing work so that other agents wont pick up the same work and so that there arent too many different statuses



uv run main.py --status "AI TODO" --repo-path ~/Dev/propermesh

Its moving suuuuuuper slow for some reason. NOTE: Maybe its because i said to thourougly read the codebase. Maybe i should tone down the language a bbit and itll be faster. or maybe it just running slow idk. Its eating requests super slow though too and just moving slow



TODO: I really need to speicfy the python version in my gemini/claude file. it doesnt really know the version

Did this flow work correctly?
it gave me a plan. I went to review it and my comment was just: It looks good and you can use this url for the download link, feel free to strat building. I think its going to work fine. I assume its just getting into biulding it so I think it is working. I was just worried it was going ot iterate on the plan or something based on my feedback instead of just building it

Actually what I probably should have done is just posted the comment and then set it to plan approved. So then it would have known to go implement and it could have just read my comment as context

TODO: does `task test` work for the gemini cli? Like does it have my same environment so does it have 'task' installed? Or would it need to install task to be able to run those commands? Not sure how it works. I think its probably just acting as me when it runs stuff but idk


TODO: Is there a way to have it plan and then immedialtey build? Is that any different from just building? Like is /plan different from its build process? or would it normally do the same work of planning and then biuld? Probably same work 




TODO: How do I want this to work in a local-first system?

Ok so I want pretty much the same statuses as the previous system. I want to be able to submit a new task somehow. Probably by placing a file somewhere with the task description. One difference though is that 


I place a file with the task.
The program will pick it up and pass it to an agent to plan it. That plan artifact gets saved. Then I review the plan. Then the agent will




Ok do I want to have a conversation going in each? Or is it not needed?
I do think I want to be able to see output too

Scenarios:
- dont even know in what way i want a thing to be built: Brainstorm
- Its clear but im not really aware of that part of the system: Build a plan but also make a context.md doc taht I can read to learn about that part of the codebase before reviewing the plan
- Its clear and I know the codebase but still want a plan: Just make a plan
- So simple it doesnt even need a plan: Just build it

After it gets to PR review, a github action will automatically review it.

Should the different things be in different files each having their own directory?
Liek a task.md and then a context.md and plan.md

Where would I put feedback on the plan.md file though? Just at the bottom of each dock could be place for comments?

And like where would it ask questions? At the bottom of the task doc or its own file?

Maybe it should jsut be one doc with each thing getting its own section


TODO: I think read-only is fine for other repos

