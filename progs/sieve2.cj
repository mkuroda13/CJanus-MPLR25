int[100] p
int max
int maxrt
sync plock
max += 100
maxrt += 10
local int k = 2
  call sieve(k)
delocal int k = 2

procedure sieve(int k)
  if maxrt > k then 
    if p[k] = 0 then 
      local int n = k
        from n = k do 
          n += k
        loop 
          acquire plock
          local int t = p[n] * (maxrt - 1)
            p[n] += t
          delocal int t = p[n] * (maxrt-1) /maxrt
          p[n] += k
          wait plock
        until n >= maxrt
        call dosieve(k,n), nextsieve(k)
      delocal int n = ((maxrt-1)/k)*k+k
    else 
      local int nk = k+1
        call sieve(nk)
      delocal int nk = k+1
    fi p[k] = 0
  else skip fi maxrt > k


procedure nextsieve(int k)
  local int nk = k+1
  call sieve(nk)
  delocal int nk = k+1


procedure dosieve(int k, int g)
  local int n = g
    from n = g do
      n += k
    loop 
      acquire plock
      local int t = p[n] * (maxrt - 1)
        p[n] += t
      delocal int t = p[n]*(maxrt-1)/maxrt
      p[n] += k
      wait plock
    until n >= max
  delocal int n = ((max-1)/k)*k+k


