import { Box, Flex, Text, useColorModeValue } from "@chakra-ui/react"
import React from "react"
import { HealthResponse } from "../hooks/useFetchHealth"
import { formatUptime } from "../utils/formatUptime"

function HealthFooter({ health }: Readonly<{ health: HealthResponse }>): React.ReactElement {
  const bg = useColorModeValue("gray.100", "gray.900")

  return (
    <Box
      as="footer"
      bg={bg}
      boxShadow="sm"
      padding={3}
      mt={4}
      borderRadius="md"
    >
      <Flex justifyContent="center" gap={6} flexWrap="wrap">
        <Text fontSize="sm">
          <Text as="span" fontWeight="bold">Version:</Text> {health.version}
        </Text>
        <Text fontSize="sm">
          <Text as="span" fontWeight="bold">Uptime:</Text> {formatUptime(health.uptime_seconds)}
        </Text>
        <Text fontSize="sm">
          <Text as="span" fontWeight="bold">Booted:</Text> {new Date(health.booted_at).toLocaleString()}
        </Text>
        <Text fontSize="sm">
          <Text as="span" fontWeight="bold">Stacks:</Text> {health.stacks_managed}
        </Text>
      </Flex>
    </Box>
  )
}

export default HealthFooter
