import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
	Table,
	TableHeader,
	TableColumn,
	TableBody,
	TableRow,
	TableCell,
	getKeyValue,
} from "@heroui/react";

export const Route = createFileRoute("/_auth/list")({
	component: RouteComponent,
});

export interface Blueprint {
	item_id: number;
	location_flag: string;
	location_id: number;
	quantity: number;
	runs: number;
	type_id: number;
	material_efficiency?: number;
	time_efficiency?: number;
}

function RouteComponent() {
	//`${window.location.protocol}//${window.location.hostname}:2727/login?src=${window.location.href}`

	const [blueprints, setBlueprints] = useState(new Map<string, Blueprint[]>());
	const [loading, setLoading] = useState(true);

  // Table mayhem incoming
  const [rowsPerPage, setRowsPerPage] = useState<number>(5);
  const [page, setPage] = useState<number>(1);
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [sortDescriptor, setSortDescriptor] = useState<SortDescriptor>({
    column: "name",
    direction: "ascending",
  });
  const [filterValue, setFilterValue] = useState("");
  const hasSearchFilter = Boolean(filterValue);

  const filteredItems = useMemo(()=> {
    let filteredBlueprints = [...blueprints];
    if(hasSearchFilter){
      filteredBlueprints = filteredBlueprints.filter((blueprint)=> blueprint.name.toLowerCase().includes(filterValue.toLowerCase()))
    }
    return filteredBlueprints;
  }, [blueprints, hasSearchFilter, filterValue]);

  const pages = Math.ceil(filteredItems.length /rowsPerPage) || 1;

  const items = useMemo(()=> {
    const start = (page - 1) * rowsPerPage;
    const end = start + rowsPerPage;

    return filteredItems.slice(start, end);
  }, [filteredItems, page, rowsPerPage]);

  const sortedItems = useMemo(()=>{ 
    return [...items].sort((a, b)=> {
      const first = a[sortDescriptor.column as keyof Blueprint] as number;
      const second = b[sortDescriptor.column as keyof Blueprint] as number;
      const cmp = first < second ? -1 : first > second ? 1 :0
      return sortDescriptor.direction === "descending" ? -cmp : cmp
    })
  }, [items, sortDescriptor]);

  const onNextPage = useCallback(() => {
    if (page < pages) {
      setPage(page + 1);
    }
  }, [page, pages]);

  const onPreviousPage = useCallback(() => {
    if (page > 1) {
      setPage(page - 1);
    }
  }, [page]);
  
	useEffect(() => {
		setLoading(true);
		fetch(
			`${window.location.protocol}//${window.location.hostname}:2727/api/blueprints`,
			{
				method: "GET",
				credentials: "include",
				mode: "cors",
			}
		)
			.then((response) => response.json())
			.then((bluePrints) => {
				let map = new Map(Object.entries(bluePrints));
        // TODO - FIGURE out which format to use for this whole mess
				setBlueprints(map);
			})
			.finally(() => setLoading(false));
	}, []);

	const columns = [
		{
			key: "name",
			label: "NAME",
      sortable: true
		},
    {key: "group",
      label: "GROUP",
      sortable: true
    }
	];
  // way to get item name from key - eve universe ESI?
  // time to figure out table structure
	return (
		<Table aria-label="Example table with dynamic content">
			<TableHeader columns={columns}>
				{(column) => <TableColumn key={column.key}>{column.label}</TableColumn>}
			</TableHeader>
			<TableBody emptyContent={"No rows to display."} items={blueprints}>
				{([key, value]: [string, Blueprint[]]) => (
					<TableRow key={key}>
						<TableCell>{`${value[0].type_id}`}</TableCell>
					</TableRow>
				)}
			</TableBody>
		</Table>
	);
}
