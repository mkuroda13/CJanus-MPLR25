int p[100]
int max
int maxrt
max += 100
maxrt += 10
local int k = 2
  call sieve(k)
delocal int k = 2


procedure sieve(int k)
  if maxrt >= k then
    if p[k] = 0 then 
      call dosieve(k)
      call nextsieve(k)
    else 
      call nextsieve(k)
    fi p[k] = 0
  else skip fi maxrt >= k


procedure nextsieve(int k)
  local int nk = k+1
  call sieve(nk)
  delocal int nk = k+1


procedure dosieve(int k)
  local int n = k
    from n = k do 
      n += k
    loop
      local int t = p[n] * (maxrt - 1)
        p[n] += t
      delocal int t = p[n]*(maxrt-1)/maxrt
      p[n] += k
    until n >= max
  delocal int n = ((max-1)/k)*k+k

