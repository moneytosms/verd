// Prints one random input. argv[1] is the seed; $SIZE grows from 1 to 10 so early failures are small.
#include <bits/stdc++.h>
using namespace std;

int main(int argc, char** argv) {
    mt19937 rng(atoi(argv[1]));
    auto rnd = [&](int lo, int hi) { return (int)(rng() % (hi - lo + 1)) + lo; };
    int size = getenv("SIZE") ? atoi(getenv("SIZE")) : 10;
    int n = rnd(1, size);
    cout << n << "\n";
    for (int i = 0; i < n; i++) cout << rnd(1, size) << " \n"[i == n - 1];
    // tree: for (int v = 2; v <= n; v++) cout << rnd(1, v - 1) << " " << v << "\n";
    // string: for (int i = 0; i < n; i++) cout << char('a' + rnd(0, 2));
}
