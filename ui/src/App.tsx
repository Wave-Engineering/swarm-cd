import { Container, Text } from "@chakra-ui/react"
import React, { useState } from "react"
import ConfigWarningBanner from "./components/ConfigWarningBanner"
import HeaderBar from "./components/HeaderBar"
import HealthFooter from "./components/HealthFooter"
import StatusCardList from "./components/StatusCardList"
import useFetchHealth from "./hooks/useFetchHealth"
import useFetchStatuses from "./hooks/useFetchStatuses"

function App(): React.ReactElement {
  const { statuses, error } = useFetchStatuses()
  const { health } = useFetchHealth()
  const [searchQuery, setSearchQuery] = useState("")

  return (
    <Container maxW="container.lg" mt={4}>
      {health?.config_warnings && health.config_warnings.length > 0 && (
        <ConfigWarningBanner warnings={health.config_warnings} />
      )}
      <HeaderBar onQueryChange={query => setSearchQuery(query)} error={error !== null} />
      {error === null ? (
        <StatusCardList statuses={statuses} query={searchQuery} />
      ) : (
        <Text fontSize="xl" align="center" color="red.500">
          {error}
        </Text>
      )}
      {health && <HealthFooter health={health} />}
    </Container>
  )
}

export default App
