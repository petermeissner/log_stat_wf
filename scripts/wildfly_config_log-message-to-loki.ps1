# WildFly 19 Configuration Script for Socket Handler Logging

$WildFlyHome="C:\wildfly-19.0.0.Final"
$ServerAddress="localhost"
$ServerPort=1515
$SocketBindingName = "log-message-to-loki"

# Set up paths
$jbossCli = Join-Path $WildFlyHome "bin\jboss-cli.bat"

# Create CLI commands
$commands = @(
    # "# Add socket binding",
    # "/socket-binding-group=standard-sockets/remote-destination-outbound-socket-binding=$($SocketBindingName):add(host=$($ServerAddress), port=$($ServerPort))",
    # "",
    # "# Add JSON formatter",
    # "/subsystem=logging/json-formatter=json:add(pretty-print=false, exception-output-type=formatted)",
    # "",
    "# Add socket handler",
    "/subsystem=logging/socket-handler=$($SocketBindingName)-handler:add(named-formatter=json, level=INFO, outbound-socket-binding-ref=$($SocketBindingName), protocol=TCP)",
    "",
    "# Add handler to root logger",
    "/subsystem=logging/root-logger=ROOT:add-handler(name=$($SocketBindingName)-handler)"
)

# Create temporary CLI script file
$tempScript = [System.IO.Path]::GetTempFileName()
$tempScript = [System.IO.Path]::ChangeExtension($tempScript, ".cli")

try {
    $commands | Out-File -FilePath $tempScript -Encoding ASCII
    
    & $jbossCli --connect --file=$tempScript
}

finally {
    # Clean up temporary file
    if (Test-Path $tempScript) {
        Remove-Item $tempScript -Force
    }
}