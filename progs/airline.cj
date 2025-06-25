int seat
int agent1
int agent2
seat+=3
call sell(agent1),sell(agent2)

procedure sell(int agent)
    from agent=0 do skip
    loop
        seat-=1
        agent+=1
    until seat<=0