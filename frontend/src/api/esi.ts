import { useIsFetching, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";

export interface EsiName {
  category: string;
  id: number;
  name: string;
}

type NameMap = Record<number, EsiName>;

const namesMapKey = ["esi-names-map"] as const;

type BatchKey = readonly ["esi-names", string];

function createBatchKey(ids: number[]): BatchKey {
  return ["esi-names", ids.join(",")];
}

async function fetchNameBatch(
  ids: number[],
  signal?: AbortSignal
): Promise<NameMap> {
  if (ids.length === 0) {
    return {};
  }

  const chunkSize = 1000;
  const result: EsiName[] = [];

  for (let i = 0; i < ids.length; i += chunkSize) {
    const chunk = ids.slice(i, i + chunkSize);
    const response = await fetch("https://esi.evetech.net/universe/names", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify(chunk),
      signal,
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch ESI names (${response.status})`);
    }

    const data: EsiName[] = await response.json();
    result.push(...data);
  }

  return result.reduce<NameMap>((acc, entry) => {
    acc[entry.id] = entry;
    return acc;
  }, {});
}

export function useEsiNames(ids: readonly number[]) {
  const uniqueIds = useMemo(() => {
    const unique = Array.from(new Set(ids.filter((id) => Number.isFinite(id))));
    unique.sort((a, b) => a - b);
    return unique;
  }, [ids]);

  const queryClient = useQueryClient();

  const mapQuery = useQuery<NameMap>({
    queryKey: namesMapKey,
    queryFn: async () => ({}) as NameMap,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const nameMap = useMemo(() => mapQuery.data ?? {}, [mapQuery.data]);
  const missing = useMemo(
    () => uniqueIds.filter((id) => nameMap[id] == null),
    [uniqueIds, nameMap]
  );

  const batchKey = useMemo<BatchKey>(() => createBatchKey(missing), [missing]);

  const batchQuery = useQuery<NameMap>({
    queryKey: batchKey,
    queryFn: ({ signal }) => fetchNameBatch(missing, signal),
    enabled: missing.length > 0,
    staleTime: 60 * 60 * 1000,
    gcTime: 60 * 60 * 1000,
  });

  const batchData = batchQuery.data;

  useEffect(() => {
    if (!batchData) return;
    queryClient.setQueryData<NameMap>(namesMapKey, (prev = {}) => ({
      ...prev,
      ...batchData,
    }));
  }, [batchData, queryClient]);

  const names = useMemo(() => {
    const map = new Map<number, string>();
    for (const id of uniqueIds) {
      map.set(id, nameMap[id]?.name ?? "Unknown");
    }
    return map;
  }, [uniqueIds, nameMap]);

  const isFetching = useIsFetching({ queryKey: batchKey }) > 0;

  return {
    names,
    isLoading: mapQuery.isLoading || (isFetching && missing.length > 0),
  };
}
