// length of the longest strictly increasing subsequence of a (n ints) in O(n log n); tails must hold n ints
int lis(const int *a, int n, int *tails) {
    int len = 0;
    for (int i = 0; i < n; i++) {
        int l = 0, r = len;
        while (l < r) {
            int mid = (l + r) / 2;
            if (tails[mid] < a[i]) l = mid + 1;
            else r = mid;
        }
        tails[l] = a[i];
        if (l == len) len++;
    }
    return len;
}
