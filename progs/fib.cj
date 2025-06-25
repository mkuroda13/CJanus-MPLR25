int n
int r
n += 10
call fib(n, r)

procedure fib(int n, int r)
    if n <= 2 then 
        r += 1
     else 
        local int l = n-1
        local int m = n-2
        call fib(l,r), fib(m,r)
        delocal int m = n-2
        delocal int l = n-1
     fi n <= 2

