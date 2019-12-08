
export function debounce<T>(f: (a: T) => void, interval: number): (a: T) => void {
    let tc: any | undefined;

    return (a: T) => {
        if (tc !== undefined) {
            clearTimeout(tc);
        }
        tc = setTimeout(() => f(a), 200);
    }
}
