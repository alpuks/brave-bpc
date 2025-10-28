import { useQuery, type UseQueryResult } from "@tanstack/react-query";

export interface Blueprint {
  quantity: number;
  runs: number;
  type_id: number;
  material_efficiency?: number;
  time_efficiency?: number;
  key: string;
}

export interface BlueprintGroup {
  type_name: string;
  blueprints: Blueprint[];
}

export const blueprintsQueryKey = ["blueprints"] as const;

async function fetchBlueprints(
  signal?: AbortSignal
): Promise<BlueprintGroup[]> {
  const response = await fetch("/api/blueprints", {
    method: "GET",
    credentials: "include",
    signal,
  });

  if (!response.ok) {
    throw new Error(`Failed to load blueprints (${response.status})`);
  }

  const data: BlueprintGroup[] = await response.json();

  return data.map((group) => ({
    type_name: group.type_name,
    blueprints: group.blueprints.map((blueprint) => ({
      ...blueprint,
      key: `${blueprint.type_id}-${blueprint.material_efficiency ?? 0}-${
        blueprint.time_efficiency ?? 0
      }-${blueprint.runs}`,
    })),
  }));
}

export function useBlueprintsQuery(): UseQueryResult<BlueprintGroup[]> {
  return useQuery({
    queryKey: blueprintsQueryKey,
    queryFn: ({ signal }) => fetchBlueprints(signal),
  });
}
