import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";

export interface BlueprintLineItem {
  type_id: number;
  runs: number;
  material_efficiency?: number;
  time_efficiency?: number;
  quantity?: number;
  type_name?: string;
}

export interface BlueprintRequest {
  id: number;
  character_id: number;
  character_name: string;
  status?: string;
  created_at: string;
  updated_at: string;
  updated_by?: string;
  public_notes?: string;
  blueprints: BlueprintLineItem[];
}

export interface CreateRequisitionBlueprint {
  type_id: number;
  runs: number;
  material_efficiency: number;
  time_efficiency: number;
  quantity: number;
}

export interface CreateRequisitionPayload {
  blueprints: CreateRequisitionBlueprint[];
}

export const requisitionsQueryKey = ["requisitions"] as const;

export async function fetchRequisitions(signal?: AbortSignal) {
  const response = await fetch("/api/requisition", {
    credentials: "include",
    signal,
  });

  if (!response.ok) {
    throw new Error(`Failed to load requisitions (${response.status})`);
  }

  const data: BlueprintRequest[] = await response.json();
  return data;
}

export function useRequisitionsQuery(): UseQueryResult<BlueprintRequest[]> {
  return useQuery({
    queryKey: requisitionsQueryKey,
    queryFn: ({ signal }) => fetchRequisitions(signal),
  });
}

async function postRequisition(payload: CreateRequisitionPayload) {
  const response = await fetch("/api/requisition", {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    throw new Error(`Failed to create requisition (${response.status})`);
  }

  return response.json().catch(() => undefined);
}

export function useCreateRequisitionMutation(): UseMutationResult<
  unknown,
  Error,
  CreateRequisitionPayload
> {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: postRequisition,
    onSuccess: () => invalidateRequisitions(queryClient),
  });
}

export function invalidateRequisitions(queryClient: QueryClient) {
  queryClient.invalidateQueries({ queryKey: requisitionsQueryKey });
}
