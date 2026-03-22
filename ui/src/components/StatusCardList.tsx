import { Text } from "@chakra-ui/react"
import React, { useEffect, useState } from "react"
import { StackStatus } from "../hooks/useFetchStatuses"
import StatusCard from "./StatusCard"

function StatusCardList({ statuses, query }: Readonly<{ statuses: StackStatus[]; query: string }>): React.ReactElement {
  const [filteredStatuses, setFilteredStatuses] = useState<StackStatus[]>(statuses)

  useEffect(() => {
    const filtered = statuses.filter(status =>
      Object.values(status).some(value => value.toString().toLowerCase().includes(query.toLowerCase()))
    )
    setFilteredStatuses(filtered)
  }, [statuses, query])

  return (
    <>
      {filteredStatuses.length === 0 ? (
        <Text fontSize="xl" align="center" mt={4}>
          No items available
        </Text>
      ) : (
        filteredStatuses.map((item, index) => (
          <StatusCard
            key={index}
            name={item.name}
            error={item.error}
            revision={item.revision}
            repo_url={item.repo_url}
            ref_type={item.ref_type}
            ref_value={item.ref_value}
            last_change_at={item.last_change_at}
            last_deployed_at={item.last_deployed_at}
          />
        ))
      )}
    </>
  )
}

export default StatusCardList
