import { JobPhase, JobPhaseMap } from "../api/werft_pb";

export function debounce<T>(f: (a: T) => void, interval: number): (a: T) => void {
    let tc: any | undefined;

    return (a: T) => {
        if (tc !== undefined) {
            clearTimeout(tc);
        }
        tc = setTimeout(() => f(a), 200);
    }
}

export function phaseToString(p: JobPhaseMap[keyof JobPhaseMap]): string {
    const kvs = Object.getOwnPropertyNames(JobPhase).map(k => [k, (JobPhase as any)[k]]).find(kv => kv[1] === p);
    return kvs![0].split("_")[1].toLowerCase();
}