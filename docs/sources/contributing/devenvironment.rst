:title: Setting Up a Dev Environment
:description: Guides on how to contribute to docker
:keywords: Docker, documentation, developers, contributing, dev environment

Setting Up a Dev Environment
^^^^^^^^^^^^^^^^^^^^^^^^^^^^

To make it easier to contribute to Docker, we provide a standard
development environment. It is important that the same environment be
used for all tests, builds and releases. The standard development
environment defines all build dependencies: system libraries and
binaries, go environment, go dependencies, etc.


Step 1: Install Docker
----------------------

Docker's build environment itself is a Docker container, so the first
step is to install Docker on your system.

You can follow the `install instructions most relevant to your system
<https://docs.docker.io/en/latest/installation/>`_.  Make sure you have
a working, up-to-date docker installation, then continue to the next
step.


Step 2: Check out the Source
----------------------------

.. code-block:: bash

    git clone http://git@github.com/dotcloud/docker
    cd docker

To checkout a different revision just use ``git checkout`` with the name of branch or revision number.


Step 3: Build the Environment
-----------------------------

This following command will build a development environment using the Dockerfile in the current directory. Essentially, it will install all the build and runtime dependencies necessary to build and test Docker. This command will take some time to complete when you first execute it.

.. code-block:: bash

    sudo docker build -t docker .



If the build is successful, congratulations! You have produced a clean build of docker, neatly encapsulated in a standard build environment. 


Step 4: Build the Docker Binary
-------------------------------

To create the Docker binary, run this command:

.. code-block:: bash

	sudo docker run -privileged -v `pwd`:/go/src/github.com/dotcloud/docker docker hack/make.sh binary

This will create the Docker binary in ``./bundles/<version>-dev/binary/``


Step 5: Run the Tests
---------------------

To execute the test cases, run this command:

.. code-block:: bash

	sudo docker run -privileged -v `pwd`:/go/src/github.com/dotcloud/docker docker hack/make.sh test


Note: if you're running the tests in vagrant, you need to specify a dns entry in the command: `-dns 8.8.8.8`

If the test are successful then the tail of the output should look something like this

.. code-block:: bash

	--- PASS: TestWriteBroadcaster (0.00 seconds)
	=== RUN TestRaceWriteBroadcaster
	--- PASS: TestRaceWriteBroadcaster (0.00 seconds)
	=== RUN TestTruncIndex
	--- PASS: TestTruncIndex (0.00 seconds)
	=== RUN TestCompareKernelVersion
	--- PASS: TestCompareKernelVersion (0.00 seconds)
	=== RUN TestHumanSize
	--- PASS: TestHumanSize (0.00 seconds)
	=== RUN TestParseHost
	--- PASS: TestParseHost (0.00 seconds)
	=== RUN TestParseRepositoryTag
	--- PASS: TestParseRepositoryTag (0.00 seconds)
	=== RUN TestGetResolvConf
	--- PASS: TestGetResolvConf (0.00 seconds)
	=== RUN TestCheckLocalDns
	--- PASS: TestCheckLocalDns (0.00 seconds)
	=== RUN TestParseRelease
	--- PASS: TestParseRelease (0.00 seconds)
	=== RUN TestDependencyGraphCircular
	--- PASS: TestDependencyGraphCircular (0.00 seconds)
	=== RUN TestDependencyGraph
	--- PASS: TestDependencyGraph (0.00 seconds)
	PASS
	ok  	github.com/dotcloud/docker/utils	0.017s




Step 6: Use Docker
-------------------

You can run an interactive session in the newly built container: 

.. code-block:: bash

	sudo docker run -privileged -i -t docker bash

	# type 'exit' to exit



.. note:: The binary is available outside the container in the directory  ``./bundles/<version>-dev/binary/``. You can swap your host docker executable with this binary for live testing - for example, on ubuntu: ``sudo service docker stop ; sudo cp $(which docker) $(which docker)_ ; sudo cp ./bundles/<version>-dev/binary/docker-<version>-dev $(which docker);sudo service docker start``.


**Need More Help?**

If you need more help then hop on to the `#docker-dev IRC channel <irc://chat.freenode.net#docker-dev>`_ or post a message on the `Docker developer mailinglist <https://groups.google.com/d/forum/docker-dev>`_.
