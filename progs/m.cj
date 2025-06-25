int x
int k
int j
int s
call sub1(),sub2()

procedure sub1()
    from k = 0 do skip loop
        if x*2 != x+x then 
            s += 1
        else skip fi x*2 != x+x
        k += 1
    until k >= 1000

procedure sub2()
    from j = 0 do skip loop
        x += 1
        j += 1
    until j >= 10000

