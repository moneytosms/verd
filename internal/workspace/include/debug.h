// verd debug.h: debug(a, b, ...) prints "[L12] a = 5, b = [1, 2, 3]" to stderr.
// Handles ints, floats, strings, pairs, tuples, optionals, any container (nested), queues, stacks, heaps.
// 2D values (matrix, grid, adjacency list, map of lists) print one row per line, "row: [..]".
// Needs C++20. Included only under -DLOCAL (verd adds it for local runs); on Codeforces debug() is a no-op.
#pragma once
#include <bits/stdc++.h>

namespace dbg_ {
using namespace std;

template <class T> string str(const T &x);

inline string str_i128(__int128 x) {
    if (x == 0) return "0";
    bool neg = x < 0;
    string s;
    for (unsigned __int128 u = neg ? -(unsigned __int128)x : x; u; u /= 10) s += char('0' + int(u % 10));
    if (neg) s += '-';
    reverse(s.begin(), s.end());
    return s;
}

template <class T> string seq(T first, T last, const char *open, const char *close) {
    string s = open;
    for (bool f = true; first != last; ++first, f = false) s += (f ? "" : ", ") + str(*first);
    return s + close;
}

template <class T> string str(const T &x) {
    if constexpr (is_same_v<T, bool>) return x ? "true" : "false";
    else if constexpr (is_same_v<T, char>) return string("'") + x + "'";
    else if constexpr (is_same_v<T, __int128>) return str_i128(x);
    else if constexpr (is_same_v<T, unsigned __int128>) return str_i128(x);
    else if constexpr (is_same_v<T, string_view>) return "\"" + string(x) + "\"";
    else if constexpr (is_convertible_v<T, string>) return "\"" + string(x) + "\"";
    else if constexpr (is_floating_point_v<T>) { ostringstream o; o << x; return o.str(); }
    else if constexpr (is_arithmetic_v<T>) return to_string(x);
    else if constexpr (is_same_v<T, vector<bool>::reference>) return x ? "true" : "false";
    else if constexpr (is_same_v<T, bitset<64>> || requires { x.to_string(); }) return x.to_string();
    else if constexpr (requires { x.has_value(); *x; }) return x.has_value() ? str(*x) : "none";
    else if constexpr (requires { x.first; x.second; }) return "(" + str(x.first) + ", " + str(x.second) + ")";
    else if constexpr (requires(T c) { c.top(); c.pop(); }) { // priority_queue / stack: print a copy, top first
        T c = x; string s = "["; bool f = true;
        for (; !c.empty(); c.pop(), f = false) s += (f ? "" : ", ") + str(c.top());
        return s + "]";
    } else if constexpr (requires(T c) { c.front(); c.pop(); }) { // queue: front first
        T c = x; string s = "["; bool f = true;
        for (; !c.empty(); c.pop(), f = false) s += (f ? "" : ", ") + str(c.front());
        return s + "]";
    } else if constexpr (requires { typename T::key_type; std::begin(x); }) return seq(std::begin(x), std::end(x), "{", "}");
    else if constexpr (requires { std::begin(x); std::end(x); }) return seq(std::begin(x), std::end(x), "[", "]");
    else if constexpr (requires { tuple_size<T>::value; }) {
        string s = "(";
        apply([&](const auto &...e) { size_t i = 0; ((s += (i++ ? ", " : "") + str(e)), ...); }, x);
        return s + ")";
    } else if constexpr (requires(ostream &o) { o << x; }) { ostringstream o; o << x; return o.str(); }
    else return "?";
}

// splits "a, f(x, y), v[2]" at top-level commas
inline vector<string> names(const string &s) {
    vector<string> r(1);
    int depth = 0;
    for (char c : s) {
        if (c == '(' || c == '[' || c == '{') depth++;
        if (c == ')' || c == ']' || c == '}') depth--;
        if (c == ',' && depth == 0) { r.emplace_back(); continue; }
        if (!(c == ' ' && r.back().empty())) r.back() += c;
    }
    return r;
}

template <class T> constexpr bool is_range = requires(const T &x) { std::begin(x); std::end(x); } && !is_convertible_v<T, string_view>;

// a 2D value (vector of vectors, grid, adjacency list, map of lists) is shown one row per line
template <class T> constexpr bool is_table() {
    if constexpr (is_range<T>) {
        using E = remove_cvref_t<decltype(*std::begin(declval<const T &>()))>;
        if constexpr (requires { typename T::mapped_type; }) return is_range<typename T::mapped_type>;
        else return is_range<E>;
    } else return false;
}

template <class T> string table(const T &t) {
    string s;
    size_t i = 0;
    for (const auto &e : t) {
        if constexpr (requires { typename T::mapped_type; }) s += "\n  " + str(e.first) + ": " + str(e.second);
        else s += "\n  " + to_string(i++) + ": " + str(e);
    }
    return s;
}

template <class T> string show(const T &x) {
    if constexpr (is_table<T>()) { string s = table(x); return s.empty() ? "[]" : s; }
    else return str(x);
}

template <class... T> void out(const char *ns, int line, const T &...xs) {
    vector<string> n = names(ns), v{show(xs)...};
    cerr << "[L" << line << "]";
    for (size_t i = 0; i < v.size(); i++) cerr << (i ? ", " : " ") << n[i] << " =" << (v[i][0] == '\n' ? "" : " ") << v[i];
    cerr << endl;
}
} // namespace dbg_

#define debug(...) dbg_::out(#__VA_ARGS__, __LINE__, __VA_ARGS__)
