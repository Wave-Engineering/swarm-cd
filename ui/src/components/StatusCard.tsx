import { Box, Grid, Link, Text, TextProps } from "@chakra-ui/react"
import React from "react"
import { formatTimestamp } from "../utils/formatTimestamp"

function StatusCard({
  name,
  error,
  revision,
  repo_url,
  ref_type,
  ref_value,
  last_change_at,
  last_deployed_at
}: Readonly<{
  name: string
  error: string
  revision: string
  repo_url: string
  ref_type: string
  ref_value: string
  last_change_at: string
  last_deployed_at: string
}>): React.ReactElement {
  return (
    <Box borderWidth="1px" borderRadius="sm" overflow="hidden" p={4} boxShadow="lg">
      <Grid templateColumns="auto 1fr" gap={2}>
        <KeyText>Name:</KeyText>
        <Text>{name}</Text>

        {error !== "" && (
          <>
            <KeyText>Error:</KeyText>
            <Text color="red.500">{error}</Text>
          </>
        )}

        <KeyText>Watching:</KeyText>
        <Text>{ref_type}:{ref_value}</Text>

        <KeyText>Revision:</KeyText>
        <Text>{revision}</Text>

        <KeyText>Repo URL:</KeyText>
        <Link color="teal.500" href={repo_url} isExternal>
          {repo_url}
        </Link>

        <KeyText>Last Change:</KeyText>
        <Text>{formatTimestamp(last_change_at)}</Text>

        <KeyText>Last Deployed:</KeyText>
        <Text>{formatTimestamp(last_deployed_at)}</Text>
      </Grid>
    </Box>
  )
}

function KeyText({ children, ...props }: Readonly<TextProps>): React.ReactElement {
  return (
    <Text fontWeight="bold" {...props}>
      {children}
    </Text>
  )
}

export default StatusCard
