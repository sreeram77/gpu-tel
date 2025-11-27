IDE Used: Intellij GoLand
AI assistant used: Windsurf/Cascade

Detailed description of the development workflow to illustrate which aspects of the workflow were accelerated / aided by AI vs manually:

1. **Project / Repo Bootstrapped**:
   - The repo was created manually and cloned. The entire requirement spec was passed on to cascade for initial analysis. The AI assistant then provided a high level architecture and design for the project. 

   - PROMPT: Analyse the requirements and provide a high level architecture and design for the project.

   - The project structure was created by the AI assistant based on the architecture and design provided.

    Notes:
    - The requirements were straightforward and AI was able to generate the code without any issues.

2**Code Bootstrapped**:
   - The code was bootstrapped by generating the initial directory structure and files using the `go mod init` command. 

   - PROMPT: Bootstrap the codebase with the required directory structure.

   - The next was to use AI to generate the makefile and the required scripts for building and running the services.

   - PROMPT: Create a makefile with minimal scripts to build and run the services locally.

   - Since the project consisted of 4 services, each service was developed independently. Asking AI to create all 4 services at once overloads it.

   - PROMPT: Based on the specs, generate the code for each message queue service. The service should use grpc for communication. Generate a proto with 2 services - Publisher and Subscriber. Both needs to use bidistreaming rpc for Publishing and subscribing. Add acknowledgement also.
   - PROMPT: Create a streamer service that streams telemetry data from CSV to the message queue. By default, it should read from test-data/metrics.csv
   - PROMPT: Create a collector service that subscribes to telemetry data from the message queue and stores it in the database.
   - PROMPT: Create an API server that provides RESTful API for querying telemetry data.

   - The AI assistant was then used to generate the required code for each service.

    Notes:
    - The progression may take a direction different from what the developer might be expecting. It may or may not be better, but the developer might be opinionated about certain things (for example, I use ENV variables for configuration, but AI opted to use a config file)
    - package versions imported by the generated code were older and led to some compatibility issues.
    - Coding patterns were sometimes inconsistent across the 4 services (example - logger). AI used different loggers in different services.

3. **Unit Test Developed**:
   - The unit test was developed by writing unit tests using the standard Go testing framework and mockery for mocking.

    Notes:
    - Initially generated tests were breaking and required multiple iterations to get them working. But Cascade was capable of running the commands inline, analyse the output and make recommendations to the code.

4. **Build Env Bootstrapped**:
   - The build environment was bootstrapped by setting up makefile for building the services and dockerfile for building the docker images.
   
   - PROMPT: Create dockerfile for each service within the cmd directory.
   - PROMPT: Create a makefile with minimal scripts to build and run the services locally.
   
   - The AI assistant initially started with using docker compose and this had to be changed to kubernetes.
   
   - PROMPT: Replace docker-compose and generate helm and kubernetes configs to run the service in kubernetes.
   - PROMPT: Add postgresql to the kubernetes configs and update the service configs for collector and api-server.
   - PROMPT: Expose api-server to the host to access the APIs. Use port forwarding and add it in the makefile.

Overall Notes:
- git tracking goes for a toss when AI is used to generate code. Since large amount of code is generated at once, LOC in each commit is huge.
- AI assistant is not always correct. It may generate code that is not compatible with the requirements or may generate code that is not correct.

